# Teleposter
Telegram poster bot
This bot posts everything you send to it to the chat you specify, adds like/dislike buttons and stores actions history in sqlite3 database.

## Build instructions
To build just run `./build.sh`, teleposter binary will appear.

## Running
Usage:
```
Usage of ./teleposter:
  -chat value
    	ChatId
  -dbname string
    	Database filename
  -proxy string
    	SOCKS5 proxy address
  -token string
    	Bot token
```
Example:
```
teleposter \
    -dbname mydb.sqlite3
    -token=XXX \
    -chat=1234
```
