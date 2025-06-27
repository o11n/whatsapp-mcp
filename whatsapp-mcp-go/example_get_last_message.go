package main

import (
	"fmt"
	"log"
	"os"
)

// Example demonstrating how to get the last message from WhatsApp
// This shows different ways to retrieve the most recent message(s)

func runExample() {
	fmt.Println("WhatsApp MCP Server - Get Last Message Example")
	fmt.Println("=============================================")

	// Example 1: Get the last message from any chat (most recent overall)
	fmt.Println("\n1. Getting the last message from any chat:")
	lastMessage, err := getLastMessageFromAnyChat()
	if err != nil {
		log.Printf("Error getting last message: %v", err)
	} else if lastMessage != "" {
		fmt.Printf("Last message: %s", lastMessage)
	} else {
		fmt.Println("No messages found")
	}

	// Example 2: Get the last interaction with a specific contact
	fmt.Println("\n2. Getting last interaction with a specific contact:")
	// Replace with an actual JID from your WhatsApp (e.g., "1234567890@s.whatsapp.net")
	contactJID := "REPLACE_WITH_ACTUAL_JID@s.whatsapp.net"
	lastInteraction, err := getLastInteraction(contactJID)
	if err != nil {
		log.Printf("Error getting last interaction: %v", err)
	} else if lastInteraction != "" {
		fmt.Printf("Last interaction: %s", lastInteraction)
	} else {
		fmt.Println("No interactions found with this contact")
	}

	// Example 3: Get the last few messages from all chats
	fmt.Println("\n3. Getting the last 5 messages from all chats:")
	recentMessages, err := getRecentMessages(5)
	if err != nil {
		log.Printf("Error getting recent messages: %v", err)
	} else {
		fmt.Println(recentMessages)
	}

	// Example 4: Get the most active chats with their last messages
	fmt.Println("\n4. Getting most active chats with their last messages:")
	activeChats, err := getActiveChattsWithLastMessages(3)
	if err != nil {
		log.Printf("Error getting active chats: %v", err)
	} else {
		printActiveChats(activeChats)
	}
}

// getLastMessageFromAnyChat gets the most recent message from any chat
func getLastMessageFromAnyChat() (string, error) {
	// Use the listMessages function with limit 1 to get the most recent message
	messages, err := listMessages(nil, nil, nil, nil, nil, 1, 0, false, 0, 0)
	if err != nil {
		return "", err
	}

	return messages, nil
}

// getRecentMessages gets the last N messages from all chats
func getRecentMessages(limit int) (string, error) {
	messages, err := listMessages(nil, nil, nil, nil, nil, limit, 0, true, 2, 2)
	if err != nil {
		return "", err
	}

	return messages, nil
}

// getActiveChattsWithLastMessages gets the most active chats with their last messages
func getActiveChattsWithLastMessages(limit int) ([]Chat, error) {
	chats, err := listChats(nil, limit, 0, true, "last_active")
	if err != nil {
		return nil, err
	}

	return chats, nil
}

// printActiveChats prints the active chats in a readable format
func printActiveChats(chats []Chat) {
	if len(chats) == 0 {
		fmt.Println("No active chats found")
		return
	}

	for i, chat := range chats {
		fmt.Printf("Chat %d:\n", i+1)

		// Chat name or JID
		if chat.Name != nil {
			fmt.Printf("  Name: %s\n", *chat.Name)
		} else {
			fmt.Printf("  JID: %s\n", chat.JID)
		}

		// Chat type
		if chat.IsGroup() {
			fmt.Printf("  Type: Group\n")
		} else {
			fmt.Printf("  Type: Direct Message\n")
		}

		// Last message time
		if chat.LastMessageTime != nil {
			fmt.Printf("  Last Active: %s\n", chat.LastMessageTime.Format("2006-01-02 15:04:05"))
		}

		// Last message content
		if chat.LastMessage != nil {
			senderInfo := "Unknown"
			if chat.LastSender != nil {
				if chat.LastIsFromMe != nil && *chat.LastIsFromMe {
					senderInfo = "You"
				} else {
					senderInfo = getSenderName(*chat.LastSender)
				}
			}
			fmt.Printf("  Last Message: [%s] %s\n", senderInfo, *chat.LastMessage)
		}

		fmt.Println()
	}
}

// Example of how you might use this with MCP tool calls
func demonstrateMCPToolCalls() {
	fmt.Println("\nMCP Tool Call Examples:")
	fmt.Println("======================")

	// Example MCP tool request for getting last messages
	fmt.Println("1. MCP Tool Call - List Messages (last 3):")
	fmt.Println(`{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "list_messages",
    "arguments": {
      "limit": 3,
      "page": 0,
      "include_context": false
    }
  }
}`)

	// Example MCP tool request for getting active chats
	fmt.Println("\n2. MCP Tool Call - List Chats (most active):")
	fmt.Println(`{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "list_chats",
    "arguments": {
      "limit": 5,
      "sort_by": "last_active",
      "include_last_message": true
    }
  }
}`)

	// Example MCP tool request for getting last interaction with a contact
	fmt.Println("\n3. MCP Tool Call - Get Last Interaction:")
	fmt.Println(`{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get_last_interaction",
    "arguments": {
      "jid": "1234567890@s.whatsapp.net"
    }
  }
}`)
}

// Example of testing the MCP server programmatically
func testMCPServerProgrammatically() {
	fmt.Println("\nProgrammatic MCP Server Test:")
	fmt.Println("=============================")

	// Note: This would require setting up the MCP client properly
	// This is just a demonstration of what the structure would look like

	// Create a mock request for testing
	mockRequest := &MockCallToolRequest{
		arguments: map[string]interface{}{
			"limit": 3.0,
			"page":  0.0,
		},
	}

	fmt.Println("Testing list_messages tool...")

	// This would be how you'd call the tool handler directly
	// In practice, you'd use the MCP client library
	result := simulateListMessagesTool(mockRequest)
	fmt.Printf("Result: %s\n", result)
}

// MockCallToolRequest simulates an MCP tool request for testing
type MockCallToolRequest struct {
	arguments map[string]interface{}
}

func (m *MockCallToolRequest) GetString(key, defaultValue string) string {
	if val, ok := m.arguments[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return defaultValue
}

func (m *MockCallToolRequest) GetFloat(key string, defaultValue float64) float64 {
	if val, ok := m.arguments[key]; ok {
		if num, ok := val.(float64); ok {
			return num
		}
	}
	return defaultValue
}

func (m *MockCallToolRequest) GetBool(key string, defaultValue bool) bool {
	if val, ok := m.arguments[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return defaultValue
}

// simulateListMessagesTool simulates calling the list_messages tool
func simulateListMessagesTool(request *MockCallToolRequest) string {
	limit := int(request.GetFloat("limit", 20))
	page := int(request.GetFloat("page", 0))
	includeContext := request.GetBool("include_context", false)
	contextBefore := int(request.GetFloat("context_before", 1))
	contextAfter := int(request.GetFloat("context_after", 1))

	messages, err := listMessages(nil, nil, nil, nil, nil, limit, page, includeContext, contextBefore, contextAfter)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	return messages
}

// Usage instructions
func printUsageInstructions() {
	fmt.Println("\nUsage Instructions:")
	fmt.Println("==================")
	fmt.Println("1. Make sure the WhatsApp bridge is running: cd ../whatsapp-bridge && go run main.go")
	fmt.Println("2. Build this example: go build -o example_get_last_message example_get_last_message.go main.go api.go")
	fmt.Println("3. Run the example: ./example_get_last_message")
	fmt.Println("")
	fmt.Println("For MCP integration:")
	fmt.Println("1. Build the MCP server: go build -o whatsapp-mcp-go main.go api.go")
	fmt.Println("2. Configure Claude/Cursor to use the whatsapp-mcp-go binary")
	fmt.Println("3. Ask Claude: 'What was my last WhatsApp message?'")
}

// Run the example if this file is executed directly
func init() {
	// Only run if this is the main program being executed
	if len(os.Args) > 0 && os.Args[0] == "./example_get_last_message" {
		runExample()
		demonstrateMCPToolCalls()
		testMCPServerProgrammatically()
		printUsageInstructions()
	}
}
