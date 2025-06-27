package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	_ "github.com/mattn/go-sqlite3"
)

// Database path relative to the binary location
func getMessagesDBPath() string {
	execPath, err := os.Executable()
	if err != nil {
		log.Printf("Failed to get executable path: %v", err)
		// Fallback to current working directory
		return filepath.Join("..", "whatsapp-bridge", "store", "messages.db")
	}

	execDir := filepath.Dir(execPath)
	return filepath.Join(execDir, "..", "whatsapp-bridge", "store", "messages.db")
}

var WHATSAPP_API_BASE_URL = "http://localhost:8080/api"

type Message struct {
	Timestamp time.Time `json:"timestamp"`
	Sender    string    `json:"sender"`
	Content   string    `json:"content"`
	IsFromMe  bool      `json:"is_from_me"`
	ChatJID   string    `json:"chat_jid"`
	ID        string    `json:"id"`
	ChatName  *string   `json:"chat_name,omitempty"`
	MediaType *string   `json:"media_type,omitempty"`
}

type Chat struct {
	JID             string     `json:"jid"`
	Name            *string    `json:"name"`
	LastMessageTime *time.Time `json:"last_message_time"`
	LastMessage     *string    `json:"last_message,omitempty"`
	LastSender      *string    `json:"last_sender,omitempty"`
	LastIsFromMe    *bool      `json:"last_is_from_me,omitempty"`
}

func (c *Chat) IsGroup() bool {
	return strings.HasSuffix(c.JID, "@g.us")
}

type Contact struct {
	PhoneNumber string  `json:"phone_number"`
	Name        *string `json:"name"`
	JID         string  `json:"jid"`
}

type MessageContext struct {
	Message Message   `json:"message"`
	Before  []Message `json:"before"`
	After   []Message `json:"after"`
}

// Database helper functions
func openDB() (*sql.DB, error) {
	// Get absolute path to the database
	messagesDBPath := getMessagesDBPath()
	absPath, err := filepath.Abs(messagesDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %v", err)
	}

	db, err := sql.Open("sqlite3", absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	return db, nil
}

func getSenderName(senderJID string) string {
	db, err := openDB()
	if err != nil {
		log.Printf("Database error while getting sender name: %v", err)
		return senderJID
	}
	defer db.Close()

	// First try matching by exact JID
	var name string
	err = db.QueryRow("SELECT name FROM chats WHERE jid = ? LIMIT 1", senderJID).Scan(&name)
	if err == nil && name != "" {
		return name
	}

	// If no result, try looking for the number within JIDs
	var phonePart string
	if strings.Contains(senderJID, "@") {
		phonePart = strings.Split(senderJID, "@")[0]
	} else {
		phonePart = senderJID
	}

	err = db.QueryRow("SELECT name FROM chats WHERE jid LIKE ? LIMIT 1", "%"+phonePart+"%").Scan(&name)
	if err == nil && name != "" {
		return name
	}

	return senderJID
}

func formatMessage(message Message, showChatInfo bool) string {
	output := ""

	if showChatInfo && message.ChatName != nil {
		output += fmt.Sprintf("[%s] Chat: %s ", message.Timestamp.Format("2006-01-02 15:04:05"), *message.ChatName)
	} else {
		output += fmt.Sprintf("[%s] ", message.Timestamp.Format("2006-01-02 15:04:05"))
	}

	contentPrefix := ""
	if message.MediaType != nil && *message.MediaType != "" {
		contentPrefix = fmt.Sprintf("[%s - Message ID: %s - Chat JID: %s] ", *message.MediaType, message.ID, message.ChatJID)
	}

	senderName := "Me"
	if !message.IsFromMe {
		senderName = getSenderName(message.Sender)
	}

	output += fmt.Sprintf("From: %s: %s%s\n", senderName, contentPrefix, message.Content)
	return output
}

func formatMessagesList(messages []Message, showChatInfo bool) string {
	output := ""
	if len(messages) == 0 {
		return "No messages to display."
	}

	for _, message := range messages {
		output += formatMessage(message, showChatInfo)
	}
	return output
}

// Tool implementations
func searchContacts(query string) ([]Contact, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	searchPattern := "%" + query + "%"

	rows, err := db.Query(`
		SELECT DISTINCT jid, name
		FROM chats
		WHERE 
			(LOWER(name) LIKE LOWER(?) OR LOWER(jid) LIKE LOWER(?))
			AND jid NOT LIKE '%@g.us'
		ORDER BY name, jid
		LIMIT 50
	`, searchPattern, searchPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []Contact
	for rows.Next() {
		var contact Contact
		var name sql.NullString

		err := rows.Scan(&contact.JID, &name)
		if err != nil {
			return nil, err
		}

		contact.PhoneNumber = strings.Split(contact.JID, "@")[0]
		if name.Valid {
			contact.Name = &name.String
		}

		contacts = append(contacts, contact)
	}

	return contacts, nil
}

func listMessages(after, before, senderPhoneNumber, chatJID, query *string, limit, page int, includeContext bool, contextBefore, contextAfter int) (string, error) {
	db, err := openDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	// Build base query
	queryParts := []string{
		"SELECT messages.timestamp, messages.sender, chats.name, messages.content, messages.is_from_me, chats.jid, messages.id, messages.media_type",
		"FROM messages",
		"JOIN chats ON messages.chat_jid = chats.jid",
	}

	var whereClauses []string
	var params []interface{}

	// Add filters
	if after != nil {
		afterTime, err := time.Parse(time.RFC3339, *after)
		if err != nil {
			return "", fmt.Errorf("invalid date format for 'after': %s. Please use ISO-8601 format", *after)
		}
		whereClauses = append(whereClauses, "messages.timestamp > ?")
		params = append(params, afterTime)
	}

	if before != nil {
		beforeTime, err := time.Parse(time.RFC3339, *before)
		if err != nil {
			return "", fmt.Errorf("invalid date format for 'before': %s. Please use ISO-8601 format", *before)
		}
		whereClauses = append(whereClauses, "messages.timestamp < ?")
		params = append(params, beforeTime)
	}

	if senderPhoneNumber != nil {
		whereClauses = append(whereClauses, "messages.sender = ?")
		params = append(params, *senderPhoneNumber)
	}

	if chatJID != nil {
		whereClauses = append(whereClauses, "messages.chat_jid = ?")
		params = append(params, *chatJID)
	}

	if query != nil {
		whereClauses = append(whereClauses, "LOWER(messages.content) LIKE LOWER(?)")
		params = append(params, "%"+*query+"%")
	}

	if len(whereClauses) > 0 {
		queryParts = append(queryParts, "WHERE "+strings.Join(whereClauses, " AND "))
	}

	// Add pagination
	offset := page * limit
	queryParts = append(queryParts, "ORDER BY messages.timestamp DESC")
	queryParts = append(queryParts, "LIMIT ? OFFSET ?")
	params = append(params, limit, offset)

	rows, err := db.Query(strings.Join(queryParts, " "), params...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var messages []Message
	for rows.Next() {
		var msg Message
		var timestampStr string
		var chatName, mediaType sql.NullString

		err := rows.Scan(&timestampStr, &msg.Sender, &chatName, &msg.Content, &msg.IsFromMe, &msg.ChatJID, &msg.ID, &mediaType)
		if err != nil {
			return "", err
		}

		msg.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			return "", err
		}

		if chatName.Valid {
			msg.ChatName = &chatName.String
		}
		if mediaType.Valid {
			msg.MediaType = &mediaType.String
		}

		messages = append(messages, msg)
	}

	if includeContext && len(messages) > 0 {
		// Add context for each message
		var messagesWithContext []Message
		for _, msg := range messages {
			context, err := getMessageContext(msg.ID, contextBefore, contextAfter)
			if err != nil {
				continue
			}
			messagesWithContext = append(messagesWithContext, context.Before...)
			messagesWithContext = append(messagesWithContext, context.Message)
			messagesWithContext = append(messagesWithContext, context.After...)
		}
		return formatMessagesList(messagesWithContext, true), nil
	}

	return formatMessagesList(messages, true), nil
}

func getMessageContext(messageID string, before, after int) (*MessageContext, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	// Get the target message first
	var msg Message
	var timestampStr, chatJIDStr string
	var chatName, mediaType sql.NullString

	err = db.QueryRow(`
		SELECT messages.timestamp, messages.sender, chats.name, messages.content, messages.is_from_me, chats.jid, messages.id, messages.chat_jid, messages.media_type
		FROM messages
		JOIN chats ON messages.chat_jid = chats.jid
		WHERE messages.id = ?
	`, messageID).Scan(&timestampStr, &msg.Sender, &chatName, &msg.Content, &msg.IsFromMe, &msg.ChatJID, &msg.ID, &chatJIDStr, &mediaType)

	if err != nil {
		return nil, fmt.Errorf("message with ID %s not found", messageID)
	}

	msg.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, err
	}

	if chatName.Valid {
		msg.ChatName = &chatName.String
	}
	if mediaType.Valid {
		msg.MediaType = &mediaType.String
	}

	// Get messages before
	beforeRows, err := db.Query(`
		SELECT messages.timestamp, messages.sender, chats.name, messages.content, messages.is_from_me, chats.jid, messages.id, messages.media_type
		FROM messages
		JOIN chats ON messages.chat_jid = chats.jid
		WHERE messages.chat_jid = ? AND messages.timestamp < ?
		ORDER BY messages.timestamp DESC
		LIMIT ?
	`, chatJIDStr, timestampStr, before)
	if err != nil {
		return nil, err
	}
	defer beforeRows.Close()

	var beforeMessages []Message
	for beforeRows.Next() {
		var beforeMsg Message
		var beforeTimestampStr string
		var beforeChatName, beforeMediaType sql.NullString

		err := beforeRows.Scan(&beforeTimestampStr, &beforeMsg.Sender, &beforeChatName, &beforeMsg.Content, &beforeMsg.IsFromMe, &beforeMsg.ChatJID, &beforeMsg.ID, &beforeMediaType)
		if err != nil {
			return nil, err
		}

		beforeMsg.Timestamp, err = time.Parse(time.RFC3339, beforeTimestampStr)
		if err != nil {
			return nil, err
		}

		if beforeChatName.Valid {
			beforeMsg.ChatName = &beforeChatName.String
		}
		if beforeMediaType.Valid {
			beforeMsg.MediaType = &beforeMediaType.String
		}

		beforeMessages = append(beforeMessages, beforeMsg)
	}

	// Get messages after
	afterRows, err := db.Query(`
		SELECT messages.timestamp, messages.sender, chats.name, messages.content, messages.is_from_me, chats.jid, messages.id, messages.media_type
		FROM messages
		JOIN chats ON messages.chat_jid = chats.jid
		WHERE messages.chat_jid = ? AND messages.timestamp > ?
		ORDER BY messages.timestamp ASC
		LIMIT ?
	`, chatJIDStr, timestampStr, after)
	if err != nil {
		return nil, err
	}
	defer afterRows.Close()

	var afterMessages []Message
	for afterRows.Next() {
		var afterMsg Message
		var afterTimestampStr string
		var afterChatName, afterMediaType sql.NullString

		err := afterRows.Scan(&afterTimestampStr, &afterMsg.Sender, &afterChatName, &afterMsg.Content, &afterMsg.IsFromMe, &afterMsg.ChatJID, &afterMsg.ID, &afterMediaType)
		if err != nil {
			return nil, err
		}

		afterMsg.Timestamp, err = time.Parse(time.RFC3339, afterTimestampStr)
		if err != nil {
			return nil, err
		}

		if afterChatName.Valid {
			afterMsg.ChatName = &afterChatName.String
		}
		if afterMediaType.Valid {
			afterMsg.MediaType = &afterMediaType.String
		}

		afterMessages = append(afterMessages, afterMsg)
	}

	return &MessageContext{
		Message: msg,
		Before:  beforeMessages,
		After:   afterMessages,
	}, nil
}

func listChats(query *string, limit, page int, includeLastMessage bool, sortBy string) ([]Chat, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	queryParts := []string{`
		SELECT 
			chats.jid,
			chats.name,
			chats.last_message_time,
			messages.content as last_message,
			messages.sender as last_sender,
			messages.is_from_me as last_is_from_me
		FROM chats
	`}

	if includeLastMessage {
		queryParts = append(queryParts, `
			LEFT JOIN messages ON chats.jid = messages.chat_jid 
			AND chats.last_message_time = messages.timestamp
		`)
	}

	var whereClauses []string
	var params []interface{}

	if query != nil {
		whereClauses = append(whereClauses, "(LOWER(chats.name) LIKE LOWER(?) OR chats.jid LIKE ?)")
		params = append(params, "%"+*query+"%", "%"+*query+"%")
	}

	if len(whereClauses) > 0 {
		queryParts = append(queryParts, "WHERE "+strings.Join(whereClauses, " AND "))
	}

	// Add sorting
	orderBy := "chats.last_message_time DESC"
	if sortBy == "name" {
		orderBy = "chats.name"
	}
	queryParts = append(queryParts, "ORDER BY "+orderBy)

	// Add pagination
	offset := page * limit
	queryParts = append(queryParts, "LIMIT ? OFFSET ?")
	params = append(params, limit, offset)

	rows, err := db.Query(strings.Join(queryParts, " "), params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var chat Chat
		var name, lastMessageTimeStr, lastMessage, lastSender sql.NullString
		var lastIsFromMe sql.NullBool

		err := rows.Scan(&chat.JID, &name, &lastMessageTimeStr, &lastMessage, &lastSender, &lastIsFromMe)
		if err != nil {
			return nil, err
		}

		if name.Valid {
			chat.Name = &name.String
		}
		if lastMessageTimeStr.Valid {
			if t, err := time.Parse(time.RFC3339, lastMessageTimeStr.String); err == nil {
				chat.LastMessageTime = &t
			}
		}
		if lastMessage.Valid {
			chat.LastMessage = &lastMessage.String
		}
		if lastSender.Valid {
			chat.LastSender = &lastSender.String
		}
		if lastIsFromMe.Valid {
			chat.LastIsFromMe = &lastIsFromMe.Bool
		}

		chats = append(chats, chat)
	}

	return chats, nil
}

func getChat(chatJID string, includeLastMessage bool) (*Chat, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := `
		SELECT 
			c.jid,
			c.name,
			c.last_message_time,
			m.content as last_message,
			m.sender as last_sender,
			m.is_from_me as last_is_from_me
		FROM chats c
	`

	if includeLastMessage {
		query += `
			LEFT JOIN messages m ON c.jid = m.chat_jid 
			AND c.last_message_time = m.timestamp
		`
	}

	query += " WHERE c.jid = ?"

	var chat Chat
	var name, lastMessageTimeStr, lastMessage, lastSender sql.NullString
	var lastIsFromMe sql.NullBool

	err = db.QueryRow(query, chatJID).Scan(&chat.JID, &name, &lastMessageTimeStr, &lastMessage, &lastSender, &lastIsFromMe)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if name.Valid {
		chat.Name = &name.String
	}
	if lastMessageTimeStr.Valid {
		if t, err := time.Parse(time.RFC3339, lastMessageTimeStr.String); err == nil {
			chat.LastMessageTime = &t
		}
	}
	if lastMessage.Valid {
		chat.LastMessage = &lastMessage.String
	}
	if lastSender.Valid {
		chat.LastSender = &lastSender.String
	}
	if lastIsFromMe.Valid {
		chat.LastIsFromMe = &lastIsFromMe.Bool
	}

	return &chat, nil
}

func getDirectChatByContact(senderPhoneNumber string) (*Chat, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var chat Chat
	var name, lastMessageTimeStr, lastMessage, lastSender sql.NullString
	var lastIsFromMe sql.NullBool

	err = db.QueryRow(`
		SELECT 
			c.jid,
			c.name,
			c.last_message_time,
			m.content as last_message,
			m.sender as last_sender,
			m.is_from_me as last_is_from_me
		FROM chats c
		LEFT JOIN messages m ON c.jid = m.chat_jid 
			AND c.last_message_time = m.timestamp
		WHERE c.jid LIKE ? AND c.jid NOT LIKE '%@g.us'
		LIMIT 1
	`, "%"+senderPhoneNumber+"%").Scan(&chat.JID, &name, &lastMessageTimeStr, &lastMessage, &lastSender, &lastIsFromMe)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	if name.Valid {
		chat.Name = &name.String
	}
	if lastMessageTimeStr.Valid {
		if t, err := time.Parse(time.RFC3339, lastMessageTimeStr.String); err == nil {
			chat.LastMessageTime = &t
		}
	}
	if lastMessage.Valid {
		chat.LastMessage = &lastMessage.String
	}
	if lastSender.Valid {
		chat.LastSender = &lastSender.String
	}
	if lastIsFromMe.Valid {
		chat.LastIsFromMe = &lastIsFromMe.Bool
	}

	return &chat, nil
}

func getContactChats(jid string, limit, page int) ([]Chat, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT DISTINCT
			c.jid,
			c.name,
			c.last_message_time,
			m.content as last_message,
			m.sender as last_sender,
			m.is_from_me as last_is_from_me
		FROM chats c
		JOIN messages m ON c.jid = m.chat_jid
		WHERE m.sender = ? OR c.jid = ?
		ORDER BY c.last_message_time DESC
		LIMIT ? OFFSET ?
	`, jid, jid, limit, page*limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []Chat
	for rows.Next() {
		var chat Chat
		var name, lastMessageTimeStr, lastMessage, lastSender sql.NullString
		var lastIsFromMe sql.NullBool

		err := rows.Scan(&chat.JID, &name, &lastMessageTimeStr, &lastMessage, &lastSender, &lastIsFromMe)
		if err != nil {
			return nil, err
		}

		if name.Valid {
			chat.Name = &name.String
		}
		if lastMessageTimeStr.Valid {
			if t, err := time.Parse(time.RFC3339, lastMessageTimeStr.String); err == nil {
				chat.LastMessageTime = &t
			}
		}
		if lastMessage.Valid {
			chat.LastMessage = &lastMessage.String
		}
		if lastSender.Valid {
			chat.LastSender = &lastSender.String
		}
		if lastIsFromMe.Valid {
			chat.LastIsFromMe = &lastIsFromMe.Bool
		}

		chats = append(chats, chat)
	}

	return chats, nil
}

func getLastInteraction(jid string) (string, error) {
	db, err := openDB()
	if err != nil {
		return "", err
	}
	defer db.Close()

	var msg Message
	var timestampStr string
	var chatName, mediaType sql.NullString

	err = db.QueryRow(`
		SELECT 
			m.timestamp,
			m.sender,
			c.name,
			m.content,
			m.is_from_me,
			c.jid,
			m.id,
			m.media_type
		FROM messages m
		JOIN chats c ON m.chat_jid = c.jid
		WHERE m.sender = ? OR c.jid = ?
		ORDER BY m.timestamp DESC
		LIMIT 1
	`, jid, jid).Scan(&timestampStr, &msg.Sender, &chatName, &msg.Content, &msg.IsFromMe, &msg.ChatJID, &msg.ID, &mediaType)

	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	msg.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return "", err
	}

	if chatName.Valid {
		msg.ChatName = &chatName.String
	}
	if mediaType.Valid {
		msg.MediaType = &mediaType.String
	}

	return formatMessage(msg, false), nil
}

func main() {
	// Create MCP server
	s := server.NewMCPServer(
		"whatsapp",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register search_contacts tool
	searchContactsTool := mcp.NewTool("search_contacts",
		mcp.WithDescription("Search WhatsApp contacts by name or phone number."),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search term to match against contact names or phone numbers"),
		),
	)
	s.AddTool(searchContactsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := request.GetString("query", "")
		if query == "" {
			return mcp.NewToolResultError("query parameter is required"), nil
		}

		contacts, err := searchContacts(query)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Database error: %v", err)), nil
		}

		content, err := json.Marshal(contacts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	// Register list_messages tool
	listMessagesTool := mcp.NewTool("list_messages",
		mcp.WithDescription("Get WhatsApp messages matching specified criteria with optional context."),
		mcp.WithString("after", mcp.Description("Optional ISO-8601 formatted string to only return messages after this date")),
		mcp.WithString("before", mcp.Description("Optional ISO-8601 formatted string to only return messages before this date")),
		mcp.WithString("sender_phone_number", mcp.Description("Optional phone number to filter messages by sender")),
		mcp.WithString("chat_jid", mcp.Description("Optional chat JID to filter messages by chat")),
		mcp.WithString("query", mcp.Description("Optional search term to filter messages by content")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of messages to return (default 20)")),
		mcp.WithNumber("page", mcp.Description("Page number for pagination (default 0)")),
		mcp.WithBoolean("include_context", mcp.Description("Whether to include messages before and after matches (default true)")),
		mcp.WithNumber("context_before", mcp.Description("Number of messages to include before each match (default 1)")),
		mcp.WithNumber("context_after", mcp.Description("Number of messages to include after each match (default 1)")),
	)
	s.AddTool(listMessagesTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var after, before, senderPhoneNumber, chatJID, query *string
		limit := int(request.GetFloat("limit", 20))
		page := int(request.GetFloat("page", 0))
		includeContext := request.GetBool("include_context", true)
		contextBefore := int(request.GetFloat("context_before", 1))
		contextAfter := int(request.GetFloat("context_after", 1))

		if val := request.GetString("after", ""); val != "" {
			after = &val
		}
		if val := request.GetString("before", ""); val != "" {
			before = &val
		}
		if val := request.GetString("sender_phone_number", ""); val != "" {
			senderPhoneNumber = &val
		}
		if val := request.GetString("chat_jid", ""); val != "" {
			chatJID = &val
		}
		if val := request.GetString("query", ""); val != "" {
			query = &val
		}

		messages, err := listMessages(after, before, senderPhoneNumber, chatJID, query, limit, page, includeContext, contextBefore, contextAfter)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		return mcp.NewToolResultText(messages), nil
	})

	// Register list_chats tool
	listChatsTool := mcp.NewTool("list_chats",
		mcp.WithDescription("Get WhatsApp chats matching specified criteria."),
		mcp.WithString("query", mcp.Description("Optional search term to filter chats by name or JID")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of chats to return (default 20)")),
		mcp.WithNumber("page", mcp.Description("Page number for pagination (default 0)")),
		mcp.WithBoolean("include_last_message", mcp.Description("Whether to include the last message in each chat (default true)")),
		mcp.WithString("sort_by", mcp.Description("Field to sort results by, either \"last_active\" or \"name\" (default \"last_active\")")),
	)
	s.AddTool(listChatsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var query *string
		limit := int(request.GetFloat("limit", 20))
		page := int(request.GetFloat("page", 0))
		includeLastMessage := request.GetBool("include_last_message", true)
		sortBy := request.GetString("sort_by", "last_active")

		if val := request.GetString("query", ""); val != "" {
			query = &val
		}

		chats, err := listChats(query, limit, page, includeLastMessage, sortBy)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Database error: %v", err)), nil
		}

		content, err := json.Marshal(chats)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	// Register get_chat tool
	getChatTool := mcp.NewTool("get_chat",
		mcp.WithDescription("Get WhatsApp chat metadata by JID."),
		mcp.WithString("chat_jid", mcp.Required(), mcp.Description("The JID of the chat to retrieve")),
		mcp.WithBoolean("include_last_message", mcp.Description("Whether to include the last message (default true)")),
	)
	s.AddTool(getChatTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		chatJID := request.GetString("chat_jid", "")
		if chatJID == "" {
			return mcp.NewToolResultError("chat_jid parameter is required"), nil
		}

		includeLastMessage := request.GetBool("include_last_message", true)

		chat, err := getChat(chatJID, includeLastMessage)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Database error: %v", err)), nil
		}

		if chat == nil {
			return mcp.NewToolResultText("null"), nil
		}

		content, err := json.Marshal(chat)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	// Register get_direct_chat_by_contact tool
	getDirectChatTool := mcp.NewTool("get_direct_chat_by_contact",
		mcp.WithDescription("Get WhatsApp chat metadata by sender phone number."),
		mcp.WithString("sender_phone_number", mcp.Required(), mcp.Description("The phone number to search for")),
	)
	s.AddTool(getDirectChatTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		senderPhoneNumber := request.GetString("sender_phone_number", "")
		if senderPhoneNumber == "" {
			return mcp.NewToolResultError("sender_phone_number parameter is required"), nil
		}

		chat, err := getDirectChatByContact(senderPhoneNumber)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Database error: %v", err)), nil
		}

		if chat == nil {
			return mcp.NewToolResultText("null"), nil
		}

		content, err := json.Marshal(chat)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	// Register get_contact_chats tool
	getContactChatsTool := mcp.NewTool("get_contact_chats",
		mcp.WithDescription("Get all WhatsApp chats involving the contact."),
		mcp.WithString("jid", mcp.Required(), mcp.Description("The contact's JID to search for")),
		mcp.WithNumber("limit", mcp.Description("Maximum number of chats to return (default 20)")),
		mcp.WithNumber("page", mcp.Description("Page number for pagination (default 0)")),
	)
	s.AddTool(getContactChatsTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jid := request.GetString("jid", "")
		if jid == "" {
			return mcp.NewToolResultError("jid parameter is required"), nil
		}

		limit := int(request.GetFloat("limit", 20))
		page := int(request.GetFloat("page", 0))

		chats, err := getContactChats(jid, limit, page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Database error: %v", err)), nil
		}

		content, err := json.Marshal(chats)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	// Register get_last_interaction tool
	getLastInteractionTool := mcp.NewTool("get_last_interaction",
		mcp.WithDescription("Get most recent WhatsApp message involving the contact."),
		mcp.WithString("jid", mcp.Required(), mcp.Description("The JID of the contact to search for")),
	)
	s.AddTool(getLastInteractionTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jid := request.GetString("jid", "")
		if jid == "" {
			return mcp.NewToolResultError("jid parameter is required"), nil
		}

		message, err := getLastInteraction(jid)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Database error: %v", err)), nil
		}

		if message == "" {
			return mcp.NewToolResultText("null"), nil
		}

		return mcp.NewToolResultText(message), nil
	})

	// Register get_message_context tool
	getMessageContextTool := mcp.NewTool("get_message_context",
		mcp.WithDescription("Get context around a specific WhatsApp message."),
		mcp.WithString("message_id", mcp.Required(), mcp.Description("The ID of the message to get context for")),
		mcp.WithNumber("before", mcp.Description("Number of messages to include before the target message (default 5)")),
		mcp.WithNumber("after", mcp.Description("Number of messages to include after the target message (default 5)")),
	)
	s.AddTool(getMessageContextTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		messageID := request.GetString("message_id", "")
		if messageID == "" {
			return mcp.NewToolResultError("message_id parameter is required"), nil
		}

		before := int(request.GetFloat("before", 5))
		after := int(request.GetFloat("after", 5))

		context, err := getMessageContext(messageID, before, after)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Error: %v", err)), nil
		}

		content, err := json.Marshal(context)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	// Register send_message tool
	sendMessageTool := mcp.NewTool("send_message",
		mcp.WithDescription("Send a WhatsApp message to a person or group. For group chats use the JID."),
		mcp.WithString("recipient", mcp.Required(), mcp.Description("The recipient - either a phone number with country code but no + or other symbols, or a JID")),
		mcp.WithString("message", mcp.Required(), mcp.Description("The message text to send")),
	)
	s.AddTool(sendMessageTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		recipient := request.GetString("recipient", "")
		if recipient == "" {
			return mcp.NewToolResultError("recipient parameter is required"), nil
		}

		message := request.GetString("message", "")
		if message == "" {
			return mcp.NewToolResultError("message parameter is required"), nil
		}

		success, statusMessage := sendMessage(recipient, message)

		result := map[string]interface{}{
			"success": success,
			"message": statusMessage,
		}

		content, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	// Register send_file tool
	sendFileTool := mcp.NewTool("send_file",
		mcp.WithDescription("Send a file such as a picture, raw audio, video or document via WhatsApp to the specified recipient. For group messages use the JID."),
		mcp.WithString("recipient", mcp.Required(), mcp.Description("The recipient - either a phone number with country code but no + or other symbols, or a JID")),
		mcp.WithString("media_path", mcp.Required(), mcp.Description("The absolute path to the media file to send (image, video, document)")),
	)
	s.AddTool(sendFileTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		recipient := request.GetString("recipient", "")
		if recipient == "" {
			return mcp.NewToolResultError("recipient parameter is required"), nil
		}

		mediaPath := request.GetString("media_path", "")
		if mediaPath == "" {
			return mcp.NewToolResultError("media_path parameter is required"), nil
		}

		success, statusMessage := sendFile(recipient, mediaPath)

		result := map[string]interface{}{
			"success": success,
			"message": statusMessage,
		}

		content, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	// Register send_audio_message tool
	sendAudioTool := mcp.NewTool("send_audio_message",
		mcp.WithDescription("Send any audio file as a WhatsApp audio message to the specified recipient. For group messages use the JID. If it errors due to ffmpeg not being installed, use send_file instead."),
		mcp.WithString("recipient", mcp.Required(), mcp.Description("The recipient - either a phone number with country code but no + or other symbols, or a JID")),
		mcp.WithString("media_path", mcp.Required(), mcp.Description("The absolute path to the audio file to send (will be converted to Opus .ogg if it's not a .ogg file)")),
	)
	s.AddTool(sendAudioTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		recipient := request.GetString("recipient", "")
		if recipient == "" {
			return mcp.NewToolResultError("recipient parameter is required"), nil
		}

		mediaPath := request.GetString("media_path", "")
		if mediaPath == "" {
			return mcp.NewToolResultError("media_path parameter is required"), nil
		}

		success, statusMessage := sendAudioMessage(recipient, mediaPath)

		result := map[string]interface{}{
			"success": success,
			"message": statusMessage,
		}

		content, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	// Register download_media tool
	downloadMediaTool := mcp.NewTool("download_media",
		mcp.WithDescription("Download media from a WhatsApp message and get the local file path."),
		mcp.WithString("message_id", mcp.Required(), mcp.Description("The ID of the message containing the media")),
		mcp.WithString("chat_jid", mcp.Required(), mcp.Description("The JID of the chat containing the message")),
	)
	s.AddTool(downloadMediaTool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		messageID := request.GetString("message_id", "")
		if messageID == "" {
			return mcp.NewToolResultError("message_id parameter is required"), nil
		}

		chatJID := request.GetString("chat_jid", "")
		if chatJID == "" {
			return mcp.NewToolResultError("chat_jid parameter is required"), nil
		}

		filePath := downloadMedia(messageID, chatJID)

		var result map[string]interface{}
		if filePath != "" {
			result = map[string]interface{}{
				"success":   true,
				"message":   "Media downloaded successfully",
				"file_path": filePath,
			}
		} else {
			result = map[string]interface{}{
				"success": false,
				"message": "Failed to download media",
			}
		}

		content, err := json.Marshal(result)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
		}

		return mcp.NewToolResultText(string(content)), nil
	})

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
