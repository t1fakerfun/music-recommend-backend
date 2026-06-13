CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS watch_history(
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    artist VARCHAR(255) NOT NULL, -- 楽曲にアーティストは必須なので追加
    total_views INT DEFAULT 0,
    evaluation INT DEFAULT 0,
    url VARCHAR(255) NOT NULL,
    channel VARCHAR(255),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS raw_watch_logs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    artist VARCHAR(255) NOT NULL, -- 楽曲にアーティストは必須なので追加
    url VARCHAR(255) NOT NULL,
    channel VARCHAR(255),
    watched_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY unique_user_watch (user_id, url, watched_at)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS listening_logs (
    id INT AUTO_INCREMENT PRIMARY KEY,
    user_id INT NOT NULL,
    history_id INT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (history_id) REFERENCES watch_history(id) ON DELETE CASCADE,
    -- 大規模データを想定し、検索が遅くならないようにインデックスを配置
    INDEX idx_user_history (user_id, history_id)
);

INSERT INTO users (id, name, email) VALUES (1, 'shun', 'shun@example.com') ON DUPLICATE KEY UPDATE id=id;
INSERT INTO watch_history (id, user_id, title, artist, url, channel) VALUES 
(1, 1, 'Marigold', 'Aimyon', 'https://example.com/marigold', 'Aimyon Official') ON DUPLICATE KEY UPDATE id=id, artist='Aimyon';
