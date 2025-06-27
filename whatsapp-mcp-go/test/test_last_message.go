// Simple example to test getting the last WhatsApp message
// Build with: go build -o test_last_message test_last_message.go
// Run with: ./test_last_message

package test

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type SimpleMessage struct {
	Timestamp time.Time
	Sender    string
	Content   string
	IsFromMe  bool
	ChatJID   string
	ChatName  *string
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

func main() {
	fmt.Println("WhatsApp Last Message Example")
	fmt.Println("=============================")

	// Test 1: Get the very last message
	fmt.Println("\n1. Getting the most recent message:")
	lastMsg, err := getLastMessage()
	if err != nil {
		log.Printf("Error: %v", err)
		fmt.Println("   Make sure the WhatsApp bridge has been run and has some message data.")
	} else {
		fmt.Printf("   %s\n", formatMessage(*lastMsg))
	}

	// Test 2: Get the last 5 messages
	fmt.Println("\n2. Getting the last 5 messages:")
	lastMessages, err := getLastNMessages(5)
	if err != nil {
		log.Printf("Error: %v", err)
	} else if len(lastMessages) == 0 {
		fmt.Println("   No messages found.")
	} else {
		for i, msg := range lastMessages {
			fmt.Printf("   %d. %s\n", i+1, formatMessage(msg))
		}
	}

	// Test 3: Database info
	fmt.Println("\n3. Database information:")
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

	fmt.Println("\n4. How to use with MCP:")
	fmt.Println("   Once the MCP server is running, you can ask Claude:")
	fmt.Println("   - 'What was my last WhatsApp message?'")
	fmt.Println("   - 'Show me my recent WhatsApp messages'")
	fmt.Println("   - 'What are my most active WhatsApp chats?'")
}
