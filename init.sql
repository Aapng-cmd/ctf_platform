DROP DATABASE IF EXISTS site;
DROP USER IF EXISTS user_site;

-- Create database
CREATE DATABASE site;

-- Use the database
USE site;

-- Create users table
CREATE TABLE users (
    id INT AUTO_INCREMENT,
    username VARCHAR(255) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    group_type INT NOT NULL,
    score INT DEFAULT 0,
    first_blood_count INT DEFAULT 0,
    PRIMARY KEY (id)
);

-- Create categories table
CREATE TABLE categories (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE
);

-- Create tasks table
CREATE TABLE tasks (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    category_id INT NOT NULL,
    description VARCHAR(500) NOT NULL UNIQUE,
    level VARCHAR(50) NOT NULL,
    author_id INT,
    cost INT UNSIGNED NOT NULL,
    hosting VARCHAR(200) UNIQUE,
    files VARCHAR(100) UNIQUE,
    flag VARCHAR(450) NOT NULL UNIQUE,
    solution TEXT NOT NULL,
    status INT DEFAULT 0,
    readme TEXT NOT NULL,
    FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE,
    FOREIGN KEY (author_id) REFERENCES users(id) ON DELETE SET NULL
);

-- Create solved_tasks table
CREATE TABLE solved_tasks (
    user_id INT NOT NULL,
    task_id INT NOT NULL,
    first_blood_id INT NOT NULL DEFAULT 0,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
    -- UNIQUE (user_id, task_id)
);

CREATE TABLE hints (
    id INT AUTO_INCREMENT PRIMARY KEY,
    task_id INT NOT NULL,
    description TEXT NOT NULL,
    cost INT,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE TABLE user_task_costs (
    user_id INT NOT NULL,
    hint_id INT NOT NULL,
    UNIQUE (user_id, hint_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (hint_id) REFERENCES hints(id) ON DELETE CASCADE
);

DELIMITER //

CREATE TRIGGER after_task_solved
AFTER INSERT ON solved_tasks
FOR EACH ROW
BEGIN
    UPDATE users
    SET score = score + (SELECT cost FROM tasks WHERE id = NEW.task_id) 
                  - COALESCE((SELECT SUM(hints.cost) 
                              FROM hints 
                              WHERE task_id = NEW.task_id 
                              AND EXISTS (SELECT 1 
                                          FROM user_task_costs 
                                          WHERE user_id = NEW.user_id 
                                          AND hint_id = hints.id)), 0)
    WHERE id = NEW.user_id;
END; //

DELIMITER ;

DELIMITER //

CREATE TRIGGER set_first_blood_id
BEFORE INSERT ON solved_tasks
FOR EACH ROW
BEGIN
    DECLARE existing_count INT;

    -- Check if this task has been solved before
    SELECT COUNT(*) INTO existing_count
    FROM solved_tasks
    WHERE task_id = NEW.task_id;

    -- If this is the first solution, set first_blood_id
    IF existing_count = 0 THEN
        SET NEW.first_blood_id = NEW.user_id;
    END IF;
END; //

DELIMITER ;
/*
DELIMITER //

CREATE TRIGGER after_task_insert
AFTER INSERT ON tasks
FOR EACH ROW
BEGIN
    UPDATE categories
    SET amount = amount + 1
    WHERE id = NEW.category_id;
END; //

DELIMITER ;

DELIMITER //

CREATE TRIGGER after_task_delete
AFTER DELETE ON tasks
FOR EACH ROW
BEGIN
    UPDATE categories
    SET amount = amount - 1
    WHERE id = OLD.category_id;
END; //

DELIMITER ;
*/
-- Create user
CREATE USER 'user_site'@'%' IDENTIFIED BY 'password';

-- Grant privileges to the user
GRANT ALL PRIVILEGES ON site.* TO 'user_site'@'%';

INSERT INTO categories (name) VALUES ('Web'), ('PPC'), ('Steganography'), ('Reverse'), ('Pwn'), ('Crypto'), ('Misc'), ('Osint'), ('Forensics'), ('Quest');
