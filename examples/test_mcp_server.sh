#!/bin/bash

# Example script demonstrating how to call the WhatsApp MCP server
# This shows how you can interact with the server using JSON-RPC calls

echo "WhatsApp MCP Server Test Script"
echo "==============================="

# Function to call MCP server
call_mcp_tool() {
    local tool_name="$1"
    local arguments="$2"
    local id="$3"
    
    echo "Calling tool: $tool_name"
    echo "Arguments: $arguments"
    echo
    
    # Create JSON-RPC request
    local request="{\"jsonrpc\": \"2.0\", \"id\": $id, \"method\": \"tools/call\", \"params\": {\"name\": \"$tool_name\", \"arguments\": $arguments}}"
    
    # Send to MCP server and show response
    echo "$request" | timeout 10 ../whatsapp-mcp-go/whatsapp-mcp-go | jq '.' 2>/dev/null || echo "Error: Make sure the MCP server is built (cd ../whatsapp-mcp-go && go build)"
    echo
}

# Test 1: Initialize the server
echo "1. Initializing MCP server..."
echo '{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": "2024-11-05", "capabilities": {}, "clientInfo": {"name": "test", "version": "1.0.0"}}}' | timeout 5 ../whatsapp-mcp-go/whatsapp-mcp-go | jq '.' 2>/dev/null || echo "Error: Make sure the MCP server is built"
echo

# Test 2: List available tools
echo "2. Listing available tools..."
echo '{"jsonrpc": "2.0", "id": 2, "method": "tools/list", "params": {}}' | timeout 5 ../whatsapp-mcp-go/whatsapp-mcp-go | jq '.result.tools[].name' 2>/dev/null || echo "Error: Could not get tools list"
echo

# Test 3: Get last 3 messages
echo "3. Getting last 3 messages..."
call_mcp_tool "list_messages" '{"limit": 3, "page": 0, "include_context": false}' 3

# Test 4: Get active chats
echo "4. Getting most active chats..."
call_mcp_tool "list_chats" '{"limit": 3, "sort_by": "last_active", "include_last_message": true}' 4

# Test 5: Search contacts (example)
echo "5. Searching contacts (if any match 'Samuel Lang')..."
call_mcp_tool "search_contacts" '{"query": "Adriana Lang"}' 5

echo "Test completed!"
echo
echo "To use with Claude:"
echo "1. Build the MCP server: cd ../whatsapp-mcp-go && go build"
echo "2. Configure Claude with the path to whatsapp-mcp-go binary"
echo "3. Ask Claude: 'What was my last WhatsApp message?'"
