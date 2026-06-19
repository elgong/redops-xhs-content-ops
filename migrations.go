package main

import (
	"context"
	"database/sql"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			name VARCHAR(80) NOT NULL,
			account_type VARCHAR(40) NOT NULL,
			auth_status VARCHAR(40) NOT NULL,
			daily_limit INT NOT NULL DEFAULT 8,
			today_scheduled INT NOT NULL DEFAULT 0,
			last_published_at DATETIME NULL,
			exception_status VARCHAR(120) NOT NULL DEFAULT '无异常',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS keyword_tasks (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			keyword VARCHAR(120) NOT NULL,
			category VARCHAR(80) NOT NULL,
			frequency_minutes INT NOT NULL DEFAULT 60,
			sample_limit INT NOT NULL DEFAULT 30,
			status VARCHAR(40) NOT NULL,
			last_collected_at DATETIME NULL,
			created_by VARCHAR(80) NOT NULL DEFAULT '运营',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS source_posts (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			keyword_task_id BIGINT NOT NULL,
			title VARCHAR(255) NOT NULL,
			content_summary TEXT NOT NULL,
			url VARCHAR(500) NOT NULL,
			views INT NOT NULL,
			likes INT NOT NULL,
			comments INT NOT NULL,
			favorites INT NOT NULL,
			hot_score DOUBLE NOT NULL,
			published_at DATETIME NOT NULL,
			captured_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_source_posts_task (keyword_task_id),
			CONSTRAINT fk_source_posts_task FOREIGN KEY (keyword_task_id) REFERENCES keyword_tasks(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS insight_reports (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			keyword_task_id BIGINT NOT NULL,
			average_hot_score DOUBLE NOT NULL,
			title_patterns TEXT NOT NULL,
			style_tags TEXT NOT NULL,
			audience_pain_points TEXT NOT NULL,
			risk_hints TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_insight_task (keyword_task_id),
			CONSTRAINT fk_insight_task FOREIGN KEY (keyword_task_id) REFERENCES keyword_tasks(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS generated_contents (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			keyword_task_id BIGINT NOT NULL,
			account_id BIGINT NOT NULL,
			title VARCHAR(255) NOT NULL,
			body TEXT NOT NULL,
			cover_text VARCHAR(255) NOT NULL,
			tags VARCHAR(500) NOT NULL,
			risk_level VARCHAR(40) NOT NULL,
			duplicate_score DOUBLE NOT NULL,
			status VARCHAR(40) NOT NULL,
			version INT NOT NULL DEFAULT 1,
			draft_id VARCHAR(120) NOT NULL DEFAULT '',
			published_url VARCHAR(500) NOT NULL DEFAULT '',
			scheduled_at DATETIME NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_generated_status (status),
			INDEX idx_generated_task (keyword_task_id),
			CONSTRAINT fk_generated_task FOREIGN KEY (keyword_task_id) REFERENCES keyword_tasks(id),
			CONSTRAINT fk_generated_account FOREIGN KEY (account_id) REFERENCES accounts(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS review_records (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			content_id BIGINT NOT NULL,
			reviewer_id VARCHAR(80) NOT NULL,
			decision VARCHAR(40) NOT NULL,
			reject_reason TEXT NOT NULL,
			regenerate_instruction TEXT NOT NULL,
			reviewed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_review_content (content_id),
			CONSTRAINT fk_review_content FOREIGN KEY (content_id) REFERENCES generated_contents(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS publish_tasks (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			content_id BIGINT NOT NULL,
			account_id BIGINT NOT NULL,
			scheduled_at DATETIME NOT NULL,
			status VARCHAR(40) NOT NULL,
			failure_reason TEXT NOT NULL,
			published_url VARCHAR(500) NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			INDEX idx_publish_due (status, scheduled_at),
			CONSTRAINT fk_publish_content FOREIGN KEY (content_id) REFERENCES generated_contents(id),
			CONSTRAINT fk_publish_account FOREIGN KEY (account_id) REFERENCES accounts(id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS app_rules (
			id BIGINT PRIMARY KEY AUTO_INCREMENT,
			brand_voice TEXT NOT NULL,
			banned_phrases TEXT NOT NULL,
			review_rules TEXT NOT NULL,
			default_reviewer VARCHAR(80) NOT NULL,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, stmt := range stmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
