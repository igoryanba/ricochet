#!/bin/bash
# Wake IDE Chat Script for Ricochet
# This script activates the IDE and triggers a chat message

MESSAGE="${1:-Check Telegram for new messages}"
IDE_APP="${2:-Cursor}"

echo "🔔 Waking up $IDE_APP with message: $MESSAGE"

osascript <<EOF
-- Activate the IDE
tell application "$IDE_APP"
    activate
end tell

delay 0.5

-- Focus the chat input (Cmd+L is common for Cursor/VSCode chat)
tell application "System Events"
    tell process "$IDE_APP"
        -- Try Cmd+L first (common chat shortcut)
        keystroke "l" using command down
        delay 0.3
        
        -- Type the wake message
        keystroke "$MESSAGE"
        delay 0.1
        
        -- Press Enter to send
        key code 36
    end tell
end tell

return "Chat activated"
EOF

echo "✅ Done! IDE should be active now."
