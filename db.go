package main

import (
	"database/sql"
	"errors"
	"log"
	"time"
)

/*
 * DB scheme:
 *  table likes:
 *   post_id uint64
 *   reaction_type int8
 *   user_id uint64
 *  table authors
 *   post_id uint64
 *   author_id uint64
 *   timestamp uint64
 */

func (bot *tBot) openDB(dbname string) {
	var err error
	bot.db, err = sql.Open("sqlite3", dbname)
	if err != nil {
		log.Panic(err)
	}
	_, err = bot.db.Exec(`
        CREATE TABLE IF NOT EXISTS likes (
            post_id       INTEGER NOT NULL,
            reaction_type INTEGER NOT NULL,
            user_id       INTEGER NOT NULL
        );
        CREATE TABLE IF NOT EXISTS authors (
            post_id   INTEGER UNIQUE NOT NULL,
            author_id INTEGER NOT NULL,
			timestamp INTEGER NOT NULL
        );
    `)
	if err != nil {
		log.Panic(err)
	}
}

func (bot *tBot) closeDB() {
	bot.db.Close()
}

func (bot *tBot) getReactions(postId *int64) [len(reactions)]int {
	var reactions_cnt [len(reactions)]int
	if postId != nil {
		rows, err := bot.db.Query(`
            SELECT reaction_type
            FROM likes
            WHERE post_id = ?`, *postId)
		if err != nil {
			log.Panic(err)
		}
		defer rows.Close()
		for rows.Next() {
			var reaction_type int
			err := rows.Scan(&reaction_type)
			if err != nil {
				log.Panic(err)
			}
			if reaction_type < 0 || reaction_type >= len(reactions) {
				log.Panic(errors.New("Bad reaction type"))
			}
			reactions_cnt[reaction_type] += 1
		}
	}
	return reactions_cnt
}

func (bot *tBot) rememberAuthor(messageId int64, chatId int64) {
	timestamp := time.Now().Unix()
	_, err := bot.db.Exec(`
		DELETE FROM authors WHERE post_id=?;
        INSERT INTO authors (post_id, author_id, timestamp)
        VALUES(?, ?, ?)`,
		messageId, messageId, chatId, timestamp)
	if err != nil {
		log.Panic(err)
	}
}

func (bot *tBot) like(postId int64, reactionType int, userId int64, name string) {
	res, err := bot.db.Exec(`
        DELETE FROM likes
        WHERE post_id = ? AND reaction_type = ? AND user_id = ?`,
		postId, reactionType, userId)
	if err != nil {
		log.Panic(err)
	}
	likes_cnt, err := res.RowsAffected()
	if err != nil {
		log.Panic(err)
	}
	res, err = bot.db.Exec(`
        DELETE FROM likes
        WHERE post_id = ? AND user_id = ?`,
		postId, userId)
	if err != nil {
		log.Panic(err)
	}
	if likes_cnt%2 == 0 {
		_, err = bot.db.Exec(`
            INSERT INTO likes (post_id, reaction_type, user_id)
            VALUES(?, ?, ?)`,
			postId, reactionType, userId)
		log.Printf("Reaction of <%v> to %v: %v\n", name, postId, reactionType)
		if err != nil {
			log.Panic(err)
		}
	} else {
		log.Printf("Reaction of <%v> to %v: not %v\n", name, postId, reactionType)
	}
}
