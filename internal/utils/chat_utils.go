package utils

func ChatIdToFullChatId(chatId int64) int64 {
	return -1000000000000 - chatId
}
