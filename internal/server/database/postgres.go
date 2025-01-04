// internal/server/database/postgres.go
package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"textual/internal/server/models"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type DB struct {
    *sql.DB
}

func NewDB(host, port, user, password, dbname string) (*DB, error) {
    connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
        host, port, user, password, dbname)
    
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, err
    }
    
    if err = db.Ping(); err != nil {
        return nil, err
    }

    return &DB{db}, nil
}


func (db *DB) AuthenticateUser(username, password string) (*models.User, error) {
    var user models.User
    var hashedPassword string

    err := db.QueryRow(`
        SELECT id, username, password_hash, status, last_seen
        FROM users 
        WHERE username = $1
    `, username).Scan(&user.ID, &user.Username, &hashedPassword, &user.Status, &user.LastSeen)

    if err == sql.ErrNoRows {
        // Create new user
        hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
        if err != nil {
            return nil, fmt.Errorf("error hashing password: %v", err)
        }

        err = db.QueryRow(`
            INSERT INTO users (username, password_hash, status, last_seen)
            VALUES ($1, $2, 'offline', NOW())
            RETURNING id, username, status, last_seen
        `, username, string(hashedBytes)).Scan(&user.ID, &user.Username, &user.Status, &user.LastSeen)

        if err != nil {
            if pgErr, ok := err.(*pq.Error); ok && pgErr.Code == "23505" {
                return nil, fmt.Errorf("username already taken")
            }
            return nil, fmt.Errorf("error creating user: %v", err)
        }
    } else if err != nil {
        return nil, fmt.Errorf("database error: %v", err)
    } else {
        if err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)); err != nil {
            return nil, fmt.Errorf("invalid password")
        }
    }

    // Update last login and status
    _, err = db.Exec(`
        UPDATE users 
        SET last_login = NOW(),
            last_seen = NOW(),
            status = 'online'
        WHERE id = $1
    `, user.ID)

    if err !=nil{
        return nil, err
    }

    return &user, nil
}

func (db *DB) GetUser(userID string) (*models.User, error) {
    var user models.User
    err := db.QueryRow(`
        SELECT id, username, status, last_seen
        FROM users
        WHERE id = $1
    `, userID).Scan(&user.ID, &user.Username, &user.Status, &user.LastSeen)

    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("user not found")
    }
    return &user, err
}

func (db *DB) UpdateUserStatus(userID, status string) error {
    result, err := db.Exec(`
        UPDATE users
        SET status = $1,
            last_seen = NOW()
        WHERE id = $2
    `, status, userID)
    
    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if rows == 0 {
        return fmt.Errorf("user not found")
    }
    return nil
}

// Friend management methods
func (db *DB) GetFriends(userID string) ([]models.User, error) {
    rows, err := db.Query(`
        SELECT u.id, u.username, u.status, u.last_seen
        FROM users u
        JOIN friends f ON (f.user_id1 = $1 AND f.user_id2 = u.id)
           OR (f.user_id2 = $1 AND f.user_id1 = u.id)
        WHERE f.status = 'accepted'
    `, userID)
    
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var friends []models.User
    for rows.Next() {
        var friend models.User
        err := rows.Scan(&friend.ID, &friend.Username, &friend.Status, &friend.LastSeen)
        if err != nil {
            return nil, err
        }
        friends = append(friends, friend)
    }

    return friends, nil
}


func (db *DB) CreateFriendRequest(fromUserID, toUserID string) error {
    _, err := db.Exec(`
        INSERT INTO friends (user_id1, user_id2, status, created_at)
        VALUES ($1, $2, 'pending', NOW())
        ON CONFLICT (user_id1, user_id2) DO UPDATE
        SET status = 'pending',
            updated_at = NOW()
        WHERE friends.status = 'rejected'
    `, fromUserID, toUserID)
    return err
}

func (db *DB) AcceptFriendRequest(userID1, userID2 string) error {
    result, err := db.Exec(`
        UPDATE friends
        SET status = 'accepted',
            updated_at = NOW()
        WHERE ((user_id1 = $1 AND user_id2 = $2)
           OR (user_id1 = $2 AND user_id2 = $1))
        AND status = 'pending'
    `, userID1, userID2)

    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("no pending friend request found")
    }
    return err
}

// Group management methods
func (db *DB) CreateGroup(name, description, creatorID string) (*models.Group, error) {
    var group models.Group
    err := db.QueryRow(`
        WITH new_group AS (
            INSERT INTO groups (name, description, created_by)
            VALUES ($1, $2, $3)
            RETURNING id, name, description, created_by, created_at
        )
        INSERT INTO group_members (group_id, user_id, role)
        SELECT id, $3, 'admin'
        FROM new_group
        RETURNING (SELECT id FROM new_group),
                  (SELECT name FROM new_group),
                  (SELECT description FROM new_group),
                  (SELECT created_by FROM new_group),
                  (SELECT created_at FROM new_group)
    `, name, description, creatorID).Scan(
        &group.ID,
        &group.Name,
        &group.Description,
        &group.CreatedBy,
        &group.CreatedAt,
    )

    if err != nil {
        return nil, err
    }

    group.Members = []string{creatorID}
    return &group, nil
}

func (db *DB) GetGroup(groupID string) (*models.Group, error) {
    var group models.Group
    err := db.QueryRow(`
        SELECT id, name, description, created_by, created_at
        FROM groups
        WHERE id = $1 AND status != 'deleted'
    `, groupID).Scan(
        &group.ID,
        &group.Name,
        &group.Description,
        &group.CreatedBy,
        &group.CreatedAt,
    )

    if err != nil {
        return nil, err
    }

    members, err := db.GetGroupMembers(groupID)
    if err != nil {
        return nil, err
    }
    group.Members = members

    return &group, nil
}

func (db *DB) GetGroupMembers(groupID string) ([]string, error) {
    rows, err := db.Query(`
        SELECT user_id
        FROM group_members
        WHERE group_id = $1
    `, groupID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var members []string
    for rows.Next() {
        var userID string
        if err := rows.Scan(&userID); err != nil {
            return nil, err
        }
        members = append(members, userID)
    }
    return members, nil
}

func (db *DB) AddUserToGroup(userID, groupID string) error {
    _, err := db.Exec(`
        INSERT INTO group_members (group_id, user_id, role)
        VALUES ($1, $2, 'member')
        ON CONFLICT (group_id, user_id) DO NOTHING
    `, groupID, userID)
    return err
}

func (db *DB) IsGroupMember(userID, groupID string) (bool, error) {
    var exists bool
    err := db.QueryRow(`
        SELECT EXISTS(
            SELECT 1 
            FROM group_members 
            WHERE group_id = $1 AND user_id = $2
        )
    `, groupID, userID).Scan(&exists)
    return exists, err
}

// Message management methods
func (db *DB) SaveMessage(msg *models.Message) error {
    // S'assurer que le message a une date d'envoi
    if msg.SentAt.IsZero() {
        msg.SentAt = time.Now()
    }

    err := db.QueryRow(`
        INSERT INTO messages (sender_id, recipient_id, group_id, content, sent_at, status)
        VALUES ($1, $2, $3, $4, $5, 'sent')
        RETURNING id
    `, msg.SenderID, msg.RecipientID, msg.GroupID, msg.Content, msg.SentAt).Scan(&msg.ID)
    
    if err != nil {
        return fmt.Errorf("failed to save message: %v", err)
    }
    
    return nil
}

func (db *DB) GetMessages(userID string, limit int) ([]models.Message, error) {
    rows, err := db.Query(`
        SELECT messages.id, 
               messages.content, 
               messages.sender_id, 
               messages.recipient_id, 
               messages.group_id, 
               messages.sent_at,
               messages.read_at,
               users.username as sender_name
        FROM messages 
        LEFT JOIN users ON messages.sender_id = users.id
        WHERE (messages.recipient_id IS NULL AND messages.group_id IS NULL)
        ORDER BY messages.sent_at DESC
        LIMIT $1
    `, limit)
    
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []models.Message
    for rows.Next() {
        var msg models.Message
        var readAt sql.NullTime
        if err := rows.Scan(
            &msg.ID,
            &msg.Content,
            &msg.SenderID,
            &msg.RecipientID,
            &msg.GroupID,
            &msg.SentAt,
            &readAt,
            &msg.SenderName,
        ); err != nil {
            return nil, err
        }
        
        if readAt.Valid {
            msg.ReadAt = &readAt.Time
        }
        
        messages = append(messages, msg)
    }

    return messages, nil
}

func (db *DB) GetMessagesBeforeID(userID string, beforeID string, limit int) ([]models.Message, error) {
    rows, err := db.Query(`
        WITH msg AS (
            SELECT sent_at 
            FROM messages 
            WHERE id = $1
        )
        SELECT messages.id, 
               messages.content, 
               messages.sender_id, 
               messages.recipient_id, 
               messages.group_id, 
               messages.sent_at,
               messages.read_at,
               users.username as sender_name
        FROM messages 
        LEFT JOIN users ON messages.sender_id = users.id
        CROSS JOIN msg
        WHERE (messages.recipient_id IS NULL AND messages.group_id IS NULL)
        AND messages.sent_at < (SELECT sent_at FROM msg)
        ORDER BY messages.sent_at DESC
        LIMIT $2
    `, beforeID, limit)
    
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []models.Message
    for rows.Next() {
        var msg models.Message
        var readAt sql.NullTime
        if err := rows.Scan(
            &msg.ID,
            &msg.Content,
            &msg.SenderID,
            &msg.RecipientID,
            &msg.GroupID,
            &msg.SentAt,
            &readAt,
            &msg.SenderName,
        ); err != nil {
            return nil, err
        }
        
        if readAt.Valid {
            msg.ReadAt = &readAt.Time
        }
        
        messages = append(messages, msg)
    }

    return messages, nil
}


func (db *DB) MarkMessageAsRead(messageID string, userID string) error {
    result, err := db.Exec(`
        UPDATE messages 
        SET read_at = NOW(),
            status = 'read'
        WHERE id = $1 AND recipient_id = $2 AND read_at IS NULL
    `, messageID, userID)
    if err != nil {
        return err
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }

    if rows == 0 {
        return fmt.Errorf("message not found or already read")
    }
    return nil
}

func (db *DB) GetGroupMessages(groupID string) ([]models.Message, error) {
    rows, err := db.Query(`
        SELECT id, content, sender_id, sent_at, read_at, users.username as sender_name
        FROM messages
        JOIN users ON messages.sender_id = users.id
        WHERE group_id = $1
        ORDER BY sent_at DESC
        LIMIT 100
    `, groupID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var messages []models.Message
    for rows.Next() {
        var msg models.Message
        msg.GroupID = &groupID
        if err := rows.Scan(
            &msg.ID,
            &msg.Content,
            &msg.SenderID,
            &msg.SentAt,
            &msg.ReadAt,
            &msg.SenderName,
        ); err != nil {
            return nil, err
        }
        messages = append(messages, msg)
    }
    return messages, nil
}

func (db *DB) Close() error {
    return db.DB.Close()
}

func (db *DB) GetFriendList(userID string) ([]string, error) {
    rows, err := db.Query(`
        SELECT CASE 
            WHEN user_id1 = $1 THEN user_id2
            ELSE user_id1
        END as friend_id
        FROM friends
        WHERE (user_id1 = $1 OR user_id2 = $1)
        AND status = 'accepted'
    `, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get friend list: %v", err)
    }
    defer rows.Close()

    var friendIDs []string
    for rows.Next() {
        var friendID string
        if err := rows.Scan(&friendID); err != nil {
            return nil, fmt.Errorf("failed to scan friend ID: %v", err)
        }
        friendIDs = append(friendIDs, friendID)
    }

    return friendIDs, nil
}

func (db *DB) RemoveUserFromGroup(userID string, groupID string) error {
    result, err := db.Exec(`
        DELETE FROM group_members
        WHERE group_id = $1 AND user_id = $2
        AND user_id NOT IN (
            SELECT created_by
            FROM groups
            WHERE id = $1
        )
    `, groupID, userID)
    
    if err != nil {
        return fmt.Errorf("failed to remove user from group: %v", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to get affected rows: %v", err)
    }

    if rows == 0 {
        return fmt.Errorf("user not found in group or is the group creator")
    }

    return nil
}

func (db *DB) RemoveFriend(userID1 string, userID2 string) error {
    result, err := db.Exec(`
        DELETE FROM friends
        WHERE (user_id1 = $1 AND user_id2 = $2)
           OR (user_id1 = $2 AND user_id2 = $1)
        AND status = 'accepted'
    `, userID1, userID2)

    if err != nil {
        return fmt.Errorf("failed to remove friend: %v", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to get affected rows: %v", err)
    }

    if rows == 0 {
        return fmt.Errorf("friend relationship not found")
    }

    return nil
}

func (db *DB) BlockUser(userID string, blockedUserID string) error {
    _, err := db.Exec(`
        INSERT INTO friends (user_id1, user_id2, status, created_at)
        VALUES ($1, $2, 'blocked', NOW())
        ON CONFLICT (user_id1, user_id2) DO UPDATE
        SET status = 'blocked',
            updated_at = NOW()
    `, userID, blockedUserID)

    if err != nil {
        return fmt.Errorf("failed to block user: %v", err)
    }

    return nil
}

func (db *DB) GetGroupRole(userID string, groupID string) (string, error) {
    var role string
    err := db.QueryRow(`
        SELECT role
        FROM group_members
        WHERE group_id = $1 AND user_id = $2
    `, groupID, userID).Scan(&role)

    if err == sql.ErrNoRows {
        return "", fmt.Errorf("user not found in group")
    }
    if err != nil {
        return "", fmt.Errorf("failed to get group role: %v", err)
    }

    return role, nil
}

func (db *DB) UpdateGroupRole(userID string, groupID string, newRole string) error {
    // check if user is the creator of the group
    var creatorID string
    err := db.QueryRow(`
        SELECT created_by
        FROM groups
        WHERE id = $1
    `, groupID).Scan(&creatorID)

    if err != nil {
        return fmt.Errorf("failed to get group creator: %v", err)
    }

    if userID == creatorID {
        return fmt.Errorf("cannot change role of group creator")
    }

    result, err := db.Exec(`
        UPDATE group_members
        SET role = $3
        WHERE group_id = $1 AND user_id = $2
    `, groupID, userID, newRole)

    if err != nil {
        return fmt.Errorf("failed to update group role: %v", err)
    }

    rows, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("failed to get affected rows: %v", err)
    }

    if rows == 0 {
        return fmt.Errorf("user not found in group")
    }

    return nil
}

func (db *DB) GetUnreadMessageCount(userID string) (int, error) {
    var count int
    err := db.QueryRow(`
        SELECT COUNT(*)
        FROM messages
        WHERE (recipient_id = $1 OR 
              group_id IN (SELECT group_id FROM group_members WHERE user_id = $1))
        AND read_at IS NULL
        AND sender_id != $1
    `, userID).Scan(&count)

    if err != nil {
        return 0, fmt.Errorf("failed to get unread message count: %v", err)
    }

    return count, nil
}

func (db *DB) GetUserGroups(userID string) ([]models.Group, error) {
    rows, err := db.Query(`
        SELECT g.id, g.name, g.description, g.created_by, g.created_at, g.status
        FROM groups g
        JOIN group_members gm ON g.id = gm.group_id
        WHERE gm.user_id = $1 AND g.status != 'deleted'
    `, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get user groups: %v", err)
    }
    defer rows.Close()

    var groups []models.Group
    for rows.Next() {
        var group models.Group
        err := rows.Scan(
            &group.ID,
            &group.Name,
            &group.Description,
            &group.CreatedBy,
            &group.CreatedAt,
            &group.Status,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan group: %v", err)
        }

        // Get group members
        members, err := db.GetGroupMembers(group.ID)
        if err != nil {
            return nil, fmt.Errorf("failed to get group members: %v", err)
        }
        group.Members = members

        groups = append(groups, group)
    }

    return groups, nil
}

func (db *DB) GetUserByUsername(username string) (*models.User, error) {
    var user models.User
    err := db.QueryRow(`
        SELECT id, username, status, last_seen
        FROM users
        WHERE LOWER(username) = LOWER($1)
    `, username).Scan(&user.ID, &user.Username, &user.Status, &user.LastSeen)

    if err == sql.ErrNoRows {
        // Log quand l'utilisateur n'est pas trouvé
        log.Printf("User not found: %s", username)
        return nil, fmt.Errorf("user not found: %s", username)
    } else if err != nil {
        log.Printf("Database error looking for user %s: %v", username, err)
        return nil, fmt.Errorf("database error: %v", err)
    }

    return &user, nil
}



func (db *DB) GetFriendRequestUsers(requestID string) (*models.User, *models.User, error) {
    // Extract user IDs from request ID format "fr-{fromID}-{toID}-{timestamp}"
    parts := strings.Split(requestID, "-")
    if len(parts) != 4 || parts[0] != "fr" {
        return nil, nil, fmt.Errorf("invalid request ID format")
    }

    fromID := parts[1]
    toID := parts[2]

    // Get sender info
    fromUser, err := db.GetUser(fromID)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to get sender: %v", err)
    }

    // Get recipient info
    toUser, err := db.GetUser(toID)
    if err != nil {
        return nil, nil, fmt.Errorf("failed to get recipient: %v", err)
    }

    return fromUser, toUser, nil
}

func (db *DB) RejectFriendRequest(fromUserID, toUserID string) error {
    result, err := db.Exec(`
        UPDATE friends
        SET status = 'rejected',
            updated_at = NOW()
        WHERE ((user_id1 = $1 AND user_id2 = $2)
           OR (user_id1 = $2 AND user_id2 = $1))
        AND status = 'pending'
    `, fromUserID, toUserID)

    if err != nil {
        return fmt.Errorf("failed to reject friend request: %v", err)
    }

    rows, err := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("no pending friend request found")
    }
    return err
}

func (db *DB) GetPendingFriendRequests(userID string) ([]models.FriendRequest, error) {
    rows, err := db.Query(`
        SELECT f.user_id1, f.user_id2, f.created_at,
               u1.username as from_username,
               u2.username as to_username
        FROM friends f
        JOIN users u1 ON f.user_id1 = u1.id
        JOIN users u2 ON f.user_id2 = u2.id
        WHERE f.user_id2 = $1
        AND f.status = 'pending'
    `, userID)

    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var requests []models.FriendRequest
    for rows.Next() {
        var req models.FriendRequest
        err := rows.Scan(
            &req.FromUserID,
            &req.ToUserID,
            &req.CreatedAt,
            &req.FromUsername,
            &req.ToUsername,
        )
        if err != nil {
            return nil, err
        }
        requests = append(requests, req)
    }

    return requests, nil
}