// Simple example to test getting the last WhatsApp message
// This demonstrates how to use the WhatsApp MCP server functionality
//
// Build with: go build -o get_last_message get_last_message.go
// Run with: ./get_last_message

package main

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database path relative to the bridge directory
var MESSAGES_DB_PATH = filepath.Join("..", "whatsapp-bridge", "store", "messages.db")

type SimpleMessage struct {
	Timestamp time.Time
	Sender    string
	Content   string
	IsFromMe  bool
	ChatJID   string
	ChatName  *string
}

func openDB() (*sql.DB, error) {
	absPath, err := filepath.Abs(MESSAGES_DB_PATH)
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

// Get the most recent message from any chat
func getLastMessage() (*SimpleMessage, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var msg SimpleMessage
	var timestampStr string
	var chatName sql.NullString

	err = db.QueryRow(`
		SELECT 
			messages.timestamp,
			messages.sender,
			chats.name,
			messages.content,
			messages.is_from_me,
			chats.jid
		FROM messages
		JOIN chats ON messages.chat_jid = chats.jid
		ORDER BY messages.timestamp DESC
		LIMIT 1
	`).Scan(&timestampStr, &msg.Sender, &chatName, &msg.Content, &msg.IsFromMe, &msg.ChatJID)

	if err != nil {
		return nil, fmt.Errorf("no messages found: %v", err)
	}

	msg.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing timestamp: %v", err)
	}

	if chatName.Valid {
		msg.ChatName = &chatName.String
	}

	return &msg, nil
}

// Get the last N messages from any chat
func getLastNMessages(n int) ([]SimpleMessage, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT 
			messages.timestamp,
			messages.sender,
			chats.name,
			messages.content,
			messages.is_from_me,
			chats.jid
		FROM messages
		JOIN chats ON messages.chat_jid = chats.jid
		ORDER BY messages.timestamp DESC
		LIMIT ?
	`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []SimpleMessage
	for rows.Next() {
		var msg SimpleMessage
		var timestampStr string
		var chatName sql.NullString

		err := rows.Scan(&timestampStr, &msg.Sender, &chatName, &msg.Content, &msg.IsFromMe, &msg.ChatJID)
		if err != nil {
			return nil, err
		}

		msg.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
		if err != nil {
			continue
		}

		if chatName.Valid {
			msg.ChatName = &chatName.String
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// Get the most active chats
func getActiveChats(n int) ([]SimpleMessage, error) {
	db, err := openDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT 
			c.jid,
			c.name,
			c.last_message_time,
			m.content,
			m.sender,
			m.is_from_me
		FROM chats c
		LEFT JOIN messages m ON c.jid = m.chat_jid AND c.last_message_time = m.timestamp
		ORDER BY c.last_message_time DESC
		LIMIT ?
	`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []SimpleMessage
	for rows.Next() {
		var msg SimpleMessage
		var timestampStr string
		var chatName sql.NullString

		err := rows.Scan(&msg.ChatJID, &chatName, &timestampStr, &msg.Content, &msg.Sender, &msg.IsFromMe)
		if err != nil {
			continue
		}

		if timestampStr != "" {
			msg.Timestamp, err = time.Parse(time.RFC3339, timestampStr)
			if err != nil {
				continue
			}
		}

		if chatName.Valid {
			msg.ChatName = &chatName.String
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// Format a message for display
func formatMessage(msg SimpleMessage) string {
	chatInfo := "Unknown Chat"
	if msg.ChatName != nil {
		chatInfo = *msg.ChatName
	} else {
		chatInfo = msg.ChatJID
	}

	senderInfo := "Me"
	if !msg.IsFromMe {
		senderInfo = getSenderName(msg.Sender)
	}

	return fmt.Sprintf("[%s] %s - %s: %s",
		msg.Timestamp.Format("2006-01-02 15:04:05"),
		chatInfo,
		senderInfo,
		msg.Content)
}

func main() {
	fmt.Println("WhatsApp Last Message Example")
	fmt.Println("=============================")
	fmt.Printf("Database path: %s\n", MESSAGES_DB_PATH)
	fmt.Println()

	// Test 1: Get the very last message
	fmt.Println("1. Getting the most recent message:")
	lastMsg, err := getLastMessage()
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
		fmt.Println("   Make sure the WhatsApp bridge has been run and has some message data.")
	} else {
		fmt.Printf("   %s\n", formatMessage(*lastMsg))
	}

	// Test 2: Get the last 5 messages
	fmt.Println("\n2. Getting the last 5 messages:")
	lastMessages, err := getLastNMessages(5)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else if len(lastMessages) == 0 {
		fmt.Println("   No messages found.")
	} else {
		for i, msg := range lastMessages {
			fmt.Printf("   %d. %s\n", i+1, formatMessage(msg))
		}
	}

	// Test 3: Get active chats
	fmt.Println("\n3. Getting most active chats:")
	activeChats, err := getActiveChats(5)
	if err != nil {
		fmt.Printf("   Error: %v\n", err)
	} else if len(activeChats) == 0 {
		fmt.Println("   No chats found.")
	} else {
		for i, chat := range activeChats {
			chatInfo := "Unknown Chat"
			if chat.ChatName != nil {
				chatInfo = *chat.ChatName
			} else {
				chatInfo = chat.ChatJID
			}

			lastMessage := "No recent message"
			if chat.Content != "" {
				senderInfo := "Me"
				if !chat.IsFromMe {
					senderInfo = getSenderName(chat.Sender)
				}
				lastMessage = fmt.Sprintf("%s: %s", senderInfo, chat.Content)
			}

			fmt.Printf("   %d. %s - %s (Last: %s)\n",
				i+1,
				chatInfo,
				chat.Timestamp.Format("2006-01-02 15:04:05"),
				lastMessage)
		}
	}

	// Test 4: Database info
	fmt.Println("\n4. Database information:")
	db, err := openDB()
	if err != nil {
		fmt.Printf("   Error connecting to database: %v\n", err)
		fmt.Printf("   Database path: %s\n", MESSAGES_DB_PATH)
		fmt.Println("   Make sure the WhatsApp bridge is running and has created the database.")
	} else {
		defer db.Close()

		var messageCount int
		err = db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&messageCount)
		if err != nil {
			fmt.Printf("   Error counting messages: %v\n", err)
		} else {
			fmt.Printf("   Total messages in database: %d\n", messageCount)
		}

		var chatCount int
		err = db.QueryRow("SELECT COUNT(*) FROM chats").Scan(&chatCount)
		if err != nil {
			fmt.Printf("   Error counting chats: %v\n", err)
		} else {
			fmt.Printf("   Total chats in database: %d\n", chatCount)
		}
	}

	fmt.Println("\n5. Using with MCP and Claude:")
	fmt.Println("   Once the MCP server is running, you can ask Claude:")
	fmt.Println("   - 'What was my last WhatsApp message?'")
	fmt.Println("   - 'Show me my recent WhatsApp messages'")
	fmt.Println("   - 'What are my most active WhatsApp chats?'")
	fmt.Println("   - 'Send a message to [contact name]'")

	fmt.Println("\n6. MCP Tool Examples:")
	fmt.Println("   The following tools are available:")
	fmt.Println("   - list_messages: Get messages with filtering and context")
	fmt.Println("   - list_chats: Get chat list sorted by activity")
	fmt.Println("   - search_contacts: Find contacts by name or phone")
	fmt.Println("   - get_last_interaction: Get most recent message with a contact")
	fmt.Println("   - send_message: Send a text message")
	fmt.Println("   - send_file: Send a file/media")
	fmt.Println("   - download_media: Download media from messages")
}
