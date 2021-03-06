package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/net/proxy"
)

const (
	pollTimeout  = 600
	maxGroupsCnt = 5
)

var reactions = [...]string{"🍰", "🤔 ", "[|||]", "🎻"}

type MediaGroup struct {
	mediaGroupId  string
	lastMessageId int64
}

type MessageGroup struct {
	authorId      int64
	date          int64
	lastMessageId int64
}

type tBot struct {
	token         string
	chatId        int64
	db            *sql.DB
	proxyAddr     string
	idWhitelist   tIdWhitelist
	mediaGroups   []MediaGroup
	messageGroups []MessageGroup
}

type tTelegramResponse struct {
	Ok          bool
	Description string
	Result      json.RawMessage
}

type tUser struct{}
type tChat struct {
	Id         *int64
	Title      *string
	Username   *string
	First_name *string
	Last_name  *string
}

func (bot *tBot) request(method string, params map[string]interface{}) (json.RawMessage, error) {
	var client *http.Client = nil
	if bot.proxyAddr != "" {
		dialer, err := proxy.SOCKS5("tcp", bot.proxyAddr, nil, proxy.Direct)
		if err != nil {
			return nil, err
		}
		transport := &http.Transport{
			Dial: dialer.Dial,
		}
		client = &http.Client{
			Transport: transport,
		}
	} else {
		client = &http.Client{}
	}
	query_params := url.Values{}
	for k, v := range params {
		query_params.Add(k, (fmt.Sprintf("%v", v)))
	}
	req_url := url.URL{
		Scheme:   "https",
		Host:     "api.telegram.org",
		Path:     "bot" + bot.token + "/" + method,
		RawQuery: query_params.Encode(),
	}
	req := req_url.String()
	resp, err := client.Get(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	answer := tTelegramResponse{Ok: false}
	err = json.Unmarshal(body, &answer)
	if err != nil {
		return nil, err
	}
	if answer.Ok == false {
		return nil, errors.New(answer.Description)
	}
	return answer.Result, nil
}

type tUpdate struct {
	Update_id            *int64           // required
	Message              *json.RawMessage // optional
	Edited_message       *json.RawMessage // optional
	Channel_post         *json.RawMessage // optional
	Edited_channel_post  *json.RawMessage // optional
	Inline_query         interface{}      // optional TODO
	Chosen_inline_result interface{}      // optional TODO
	Callback_query       *json.RawMessage // optional TODO
	Shipping_query       interface{}      // optional TODO
	Pre_checkout_query   interface{}      // optional TODO
}

func (bot *tBot) getUpdates(offset *int64) []tUpdate {
	params := make(map[string]interface{})
	params["timeout"] = pollTimeout
	if offset != nil {
		params["offset"] = *offset
	}
	answer, err := bot.request("getUpdates", params)
	if err != nil {
		log.Panic(err)
	}
	var updates []tUpdate
	err = json.Unmarshal(answer, &updates)
	if err != nil {
		log.Panic(err)
	}
	return updates
}

func (bot *tBot) getLikeKeyboard(postId *int64) string {
	reactions_cnt := bot.getReactions(postId)
	var keyboard strings.Builder
	keyboard.WriteString(`{"inline_keyboard":[[`)
	for i, cnt := range reactions_cnt {
		if i != 0 {
			keyboard.WriteString(",")
		}
		fmt.Fprintf(&keyboard, `{"text":"%s %d","callback_data":"%d"}`,
			reactions[i], cnt, i)
	}
	keyboard.WriteString(`]]}`)
	return keyboard.String()
}

type tMessageEntity struct {
	Type   *string
	Offset *int64
	Length *int64
	Url    *string
	User   *interface{}
}
type tAnimation struct {
	File_id  string
	Width    *int64
	Height   *int64
	Duration *int64
}
type tPhoto struct {
	File_id string
}
type tVideo struct {
	File_id  string
	Width    *int64
	Height   *int64
	Duration *int64
}
type tMessage struct {
	Message_id     *int64
	Chat           *tChat
	Date           int64
	Text           *string
	Entities       []tMessageEntity
	Audio          *interface{} // TODO
	Document       *interface{} // TODO
	Animation      *tAnimation
	Media_group_id *string
	Photo          []tPhoto
	Sticker        *interface{} // TODO
	Video          *tVideo
	Voice          *interface{} // TODO
	Video_note     *interface{} // TODO
	Caption        *interface{} // TODO
	Contact        *interface{} // TODO
	Location       *interface{} // TODO
}

type tRequest struct {
	method string
	params map[string]interface{}
}

func newRequest() tRequest {
	var request tRequest
	request.params = make(map[string]interface{})
	return request
}

func (bot *tBot) unwatchPost(messageId int64) {
	params := map[string]interface{}{
		"chat_id":    bot.chatId,
		"message_id": messageId,
	}
	_, err := bot.request("editMessageReplyMarkup", params)
	if err != nil {
		log.Println(err)
	}
	bot.forgetPost(messageId)
}

func (bot *tBot) handleMediaGroup(message tMessage, outputId int64) {
	for i, group := range bot.mediaGroups {
		if group.mediaGroupId == *message.Media_group_id {
			bot.unwatchPost(group.lastMessageId)
			bot.mediaGroups[i].lastMessageId = outputId
			return
		}
	}
	if len(bot.mediaGroups) >= maxGroupsCnt {
		bot.mediaGroups = bot.mediaGroups[1:maxGroupsCnt]
		var newMediaGroups = make([]MediaGroup, len(bot.mediaGroups))
		copy(newMediaGroups, bot.mediaGroups)
		bot.mediaGroups = newMediaGroups
	}
	bot.mediaGroups = append(bot.mediaGroups,
		MediaGroup{*message.Media_group_id, outputId})
}

func (bot *tBot) handleMessageGroup(message tMessage, outputId int64) {
	var authorId int64 = *message.Chat.Id
	var date int64 = message.Date
	for i, group := range bot.messageGroups {
		if group.authorId == authorId && group.date == date {
			bot.unwatchPost(group.lastMessageId)
			bot.messageGroups[i].lastMessageId = outputId
			return
		}
	}
	if len(bot.messageGroups) >= maxGroupsCnt {
		bot.messageGroups = bot.messageGroups[1:maxGroupsCnt]
		var newMessageGroups = make([]MessageGroup, len(bot.messageGroups))
		copy(newMessageGroups, bot.messageGroups)
		bot.messageGroups = newMessageGroups
	}
	bot.messageGroups = append(bot.messageGroups,
		MessageGroup{authorId, date, outputId})
}

func (bot *tBot) handleMessage(messageJson json.RawMessage) {
	log.Println("Input message")
	var message tMessage
	err := json.Unmarshal(messageJson, &message)
	if err != nil {
		log.Panic(err)
	}
	if !bot.idWhitelist.Has(*message.Chat.Id) {
		log.Printf("Input message from %v (%v)", *message.Chat.Id, bot.getAuthorName(*message.Chat.Id))
		return
	}

	var request = newRequest()
	request.params["chat_id"] = bot.chatId
	// Handle commands. A command must start from the beginning of the message
	for _, entity := range message.Entities {
		if message.Text == nil ||
			entity.Type == nil ||
			entity.Offset == nil ||
			entity.Length == nil {
			log.Println("Malformed message.Entity")
			continue
		}
		if *entity.Offset == 0 && *entity.Type == "bot_command" {
			if int(*entity.Length) > len(*message.Text) {
				log.Println("malformed bot command")
				continue
			}
			bot.handleCommand(*message.Chat.Id,
				(*message.Text)[0:*entity.Length])
			// There can be only one command since it starts with a message
			return
		}
	}
	request.method = "copyMessage"
	request.params["from_chat_id"] = *message.Chat.Id
	request.params["message_id"] = *message.Message_id
	request.params["reply_markup"] = bot.getLikeKeyboard(nil)
	answer, err := bot.request(request.method, request.params)
	if err != nil {
		log.Panic(err)
	}
	var sentMessage tMessage
	err = json.Unmarshal(answer, &sentMessage)
	if err != nil {
		log.Panic(err)
	}
	bot.rememberAuthor(*sentMessage.Message_id, *message.Chat.Id)

	if message.Media_group_id != nil {
		bot.handleMediaGroup(message, *sentMessage.Message_id)
	} else {
		bot.handleMessageGroup(message, *sentMessage.Message_id)
	}
}

func (bot *tBot) answerCallback(id string, text string) {
	params := map[string]interface{}{
		"callback_query_id": id,
		"text":              text,
	}
	bot.request("answerCallbackQuery", params)
}

func (bot *tBot) handleCallback(callbackQueryJson json.RawMessage) {
	type tUser struct {
		Id         *int64
		First_name string
	}
	type tMessage struct {
		Message_id *int64
		Chat       *tChat
	}
	type tCallbackQuery struct {
		Id      string
		Message *tMessage
		From    *tUser
		Data    *string
	}
	var callbackQuery tCallbackQuery
	err := json.Unmarshal(callbackQueryJson, &callbackQuery)
	if err != nil {
		log.Panic(err)
	}
	// TODO check fields presence
	num, err := strconv.Atoi(*callbackQuery.Data)
	if num < 0 || num >= len(reactions) {
		log.Panic(errors.New("Bad reaction type"))
	}
	postAuthorId := bot.getPostAuthor(*callbackQuery.Message.Message_id)
	if postAuthorId == nil {
		log.Println("Trying to like non-existing post id ", *callbackQuery.Message.Message_id)
		return
	}
	if *postAuthorId == *callbackQuery.From.Id {
		log.Printf("Self reaction of <%v> to %v: %v\n",
			callbackQuery.From.First_name,
			*callbackQuery.Message.Message_id,
			num)
		bot.answerCallback(callbackQuery.Id, "That's your post")
		return
	}
	bot.like(*callbackQuery.Message.Message_id, num,
		*callbackQuery.From.Id,
		callbackQuery.From.First_name)
	params := map[string]interface{}{
		"chat_id":      *callbackQuery.Message.Chat.Id,
		"message_id":   *callbackQuery.Message.Message_id,
		"reply_markup": bot.getLikeKeyboard(callbackQuery.Message.Message_id),
	}
	bot.request("editMessageReplyMarkup", params) // TODO check answer and err
}

func (bot *tBot) handleUpdate(update tUpdate) int64 {
	if update.Update_id == nil {
		log.Panic("nil update id")
	}
	if update.Message != nil {
		bot.handleMessage(*update.Message)
	}
	if update.Callback_query != nil {
		bot.handleCallback(*update.Callback_query)
	}
	if update.Edited_message != nil ||
		update.Inline_query != nil ||
		update.Chosen_inline_result != nil ||
		update.Shipping_query != nil ||
		update.Pre_checkout_query != nil {
		log.Println("Some features of this messages are not supported by the bot yet")
	}
	return *update.Update_id
}

// TODO store result in db
func (bot *tBot) getAuthorName(chatId int64) string {
	answer, err := bot.request("getChat", map[string]interface{}{
		"chat_id": chatId,
	})
	if err != nil {
		log.Panic(err)
	}
	var chat tChat
	err = json.Unmarshal(answer, &chat)
	if err != nil {
		log.Panic(err)
	}
	if chat.Username != nil {
		ret := "@" + *chat.Username
		if chat.First_name != nil {
			ret += " [" + *chat.First_name
			if chat.Last_name != nil {
				ret += *chat.Last_name
			}
			ret += "]"
		} else if chat.Last_name != nil {
			ret += " [" + *chat.Last_name + "]"
		} else if chat.Title != nil {
			ret += " [" + *chat.Title + "]"
		}
		return ret
	}
	return strconv.FormatInt(chatId, 10)
}

type IntFlag struct {
	set   bool
	value int64
}

func (intf *IntFlag) Set(x string) error {
	var err error
	intf.value, err = strconv.ParseInt(x, 10, 64)
	if err != nil {
		return err
	}
	intf.set = true
	return nil
}

func (intf *IntFlag) String() string {
	if intf.set {
		return strconv.FormatInt(intf.value, 10)
	}
	return "nil"
}

type tIdWhitelist []int64

func (idWhitelist *tIdWhitelist) String() string {
	return fmt.Sprint(*idWhitelist)
}

func (idWhitelist *tIdWhitelist) Set(value string) error {
	v, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		*idWhitelist = append(*idWhitelist, v)
	}
	return err
}

func (idWhitelist *tIdWhitelist) Has(id int64) bool {
	for _, listed := range *idWhitelist {
		if id == listed {
			return true
		}
	}
	return false
}

func main() {
	var chatId IntFlag
	var dbname string
	var bot tBot
	flag.StringVar(&dbname, "dbname", "", "Database filename")
	flag.StringVar(&bot.token, "token", "", "Bot token")
	flag.Var(&chatId, "chat", "ChatId")
	flag.StringVar(&bot.proxyAddr, "proxy", "", "SOCKS5 proxy address")
	flag.Var(&bot.idWhitelist, "whitelist", "List of allowed posters")
	flag.Parse()
	if dbname == "" || bot.token == "" || !chatId.set {
		flag.Usage()
		return
	}
	bot.chatId = chatId.value

	bot.openDB(dbname)
	defer bot.closeDB()
	log.Println("Started serving updates")
	var poffset *int64
	var offset int64
	for {
		updates := bot.getUpdates(poffset)
		for _, update := range updates {
			offset = bot.handleUpdate(update) + 1
			poffset = &offset
		}
		if len(updates) == 0 {
			log.Println("No updates")
		}
	}
}
