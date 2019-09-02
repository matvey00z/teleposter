package main

import "fmt"

func (bot *tBot) replyWText(chatId int64, text string) {
	bot.request("sendMessage", map[string]interface{}{
		"chat_id": chatId,
		"text":    text,
	})
}

func (bot *tBot) handleCommand(chatId int64, command string) {
	if command == "/help" || command == "/start" {
		bot.help(chatId)
	} else if command == "/mystats" {
		bot.myStats(chatId)
	} else if command == "/allstats" {
		bot.allStats(chatId)
	}
}

func (bot *tBot) help(chatId int64) {
	bot.replyWText(chatId, `
Hi! It's Telegram Poster bot.
Available commands are:
    /help - show this help
    /mystats - show my stats
    /allstats - show stats for everybody
Other commands are coming :)
Read more about this bot and report bugs here: https://github.com/matvey00z/teleposter
`,
	)
}

func getStatsString(total int64, reactionsCnt [len(reactions)]int64) string {
	ret := fmt.Sprintf("%v total", total)
	for i, cnt := range reactionsCnt {
		ret += fmt.Sprintf(", %v%v", cnt, reactions[i])
	}
	return ret
}

func (bot *tBot) myStats(chatId int64) {
	totalPosts, totalReactions := bot.getUserStats(chatId)
	bot.replyWText(chatId, getStatsString(totalPosts, totalReactions))
}

func (bot *tBot) allStats(chatId int64) {
	authors := bot.getAuthorsList()
	var reply string
	for _, authorId := range authors {
		authorName := bot.getAuthorName(authorId)
		totalPosts, totalReactions := bot.getUserStats(authorId)
		reply += fmt.Sprintf("%s: %s\n", authorName,
			getStatsString(totalPosts, totalReactions))
	}
	bot.replyWText(chatId, reply)
}
