package main

func (bot *tBot) replyWText(chatId int64, text string) {
	bot.request("sendMessage", map[string]interface{}{
		"chat_id": chatId,
		"text":    text,
	})
}

func (bot *tBot) handleCommand(chatId int64, command string) {
	if command == "/help" || command == "/start" {
		bot.help(chatId)
	}
}

func (bot *tBot) help(chatId int64) {
	bot.replyWText(chatId, `
Hi! It's Telegram Poster bot.
Available commands are:
    /help - show this help
Other commands are coming :)
Read more about this bot and report bugs here: https://github.com/matvey00z/teleposter
`,
	)
}
