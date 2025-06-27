# WhatsApp MCP Examples

This directory contains examples demonstrating how to use the WhatsApp MCP server functionality.

## Examples

### `get_last_message.go`

A simple example that demonstrates how to:
- Get the most recent WhatsApp message
- Get the last N messages from any chat
- Get the most active chats with their last messages
- Check database statistics

#### Building and Running

```bash
cd examples
go build -o get_last_message get_last_message.go
./get_last_message
```

#### Sample Output

```
WhatsApp Last Message Example
=============================
Database path: ../whatsapp-bridge/store/messages.db

1. Getting the most recent message:
   [2025-06-27 09:48:42] status - status: 

2. Getting the last 5 messages:
   1. [2025-06-27 09:48:42] status - status: 
   2. [2025-06-27 07:21:34] status - status: 
   3. [2025-06-27 07:21:32] PD Verabredungen und mehr - PD Verabredungen und mehr: 
   4. [2025-06-27 06:15:55] Versammlung Technik Zoom - Versammlung Technik Zoom: Guten Morgen 😃 

3. Getting most active chats:
   1. status - 2025-06-27 09:48:42 (Last: No recent message)
   2. PD Verabredungen und mehr - 2025-06-27 07:21:32 (Last: No recent message)

4. Database information:
   Total messages in database: 4825
   Total chats in database: 414
```

## Using with MCP and Claude

Once you have the WhatsApp MCP server running and configured with Claude, you can ask questions like:

- "What was my last WhatsApp message?"
- "Show me my recent WhatsApp messages"
- "What are my most active WhatsApp chats?"
- "Send a message to [contact name]"
- "Search for messages containing 'meeting'"

## Available MCP Tools

The WhatsApp MCP server provides these tools for Claude:

### Message Retrieval
- **`list_messages`** - Get messages with filtering and context
- **`get_message_context`** - Get context around a specific message
- **`get_last_interaction`** - Get most recent message with a contact

### Chat Management
- **`list_chats`** - Get chat list sorted by activity
- **`get_chat`** - Get specific chat information
- **`get_direct_chat_by_contact`** - Find direct chat with a contact
- **`get_contact_chats`** - Get all chats involving a contact

### Contact Search
- **`search_contacts`** - Find contacts by name or phone number

### Message Sending
- **`send_message`** - Send a text message
- **`send_file`** - Send a file/media
- **`send_audio_message`** - Send audio as a voice message

### Media Handling
- **`download_media`** - Download media from messages

## Prerequisites

1. **WhatsApp Bridge Running**: Make sure the WhatsApp bridge is running with some message data:
   ```bash
   cd ../whatsapp-bridge
   go run main.go
   ```

2. **Database Available**: The examples look for the database at `../whatsapp-bridge/store/messages.db`

## Troubleshooting

If you get "no messages found" or database errors:

1. Ensure the WhatsApp bridge has been run and authenticated
2. Check that the database files exist in `../whatsapp-bridge/store/`
3. Verify you have some message history (send a test message to yourself)

## Example MCP Tool Calls

Here are example JSON-RPC calls you can make to the MCP server:

### Get Last 3 Messages
```json
{
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
}
```

### Get Most Active Chats
```json
{
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
}
```

### Send a Message
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "send_message",
    "arguments": {
      "recipient": "1234567890",
      "message": "Hello from MCP!"
    }
  }
}
```
