-- internal/server/database/migrations/002_test_data.sql

-- test file


-- Mot de passe : test123 (haché avec bcrypt)
INSERT INTO users (username, password_hash) VALUES 
('test_user', '$2a$14$ajq8Q7fbtFRQvXpdCq7Jcuy.T.PBc3/6MxgWIuHtwaZq2+hrhg.XC'),
('john_doe', '$2a$14$ajq8Q7fbtFRQvXpdCq7Jcuy.T.PBc3/6MxgWIuHtwaZq2+hrhg.XC'),
('jane_doe', '$2a$14$ajq8Q7fbtFRQvXpdCq7Jcuy.T.PBc3/6MxgWIuHtwaZq2+hrhg.XC');

-- Création d'un groupe de test
INSERT INTO groups (name, description) VALUES 
('General', 'General discussion group');


INSERT INTO group_members (group_id, user_id)
SELECT g.id, u.id
FROM groups g, users u
WHERE g.name = 'General';