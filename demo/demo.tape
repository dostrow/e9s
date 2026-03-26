# e9s demo recording
# Requires: vhs (https://github.com/charmbracelet/vhs)
# Usage: vhs demo/demo.tape
#
# Before recording, deploy the demo infrastructure:
#   cd demo && tofu init && tofu apply
#
# After recording, tear it down:
#   cd demo && tofu destroy

Output demo/e9s-demo.gif
Output demo/e9s-demo.webm

Set Shell "bash"
Set FontSize 14
Set Width 1400
Set Height 800
Set Padding 10
Set Theme "Catppuccin Mocha"
Set TypingSpeed 60ms

# Start e9s
Type "e9s"
Enter
Sleep 2s

# --- Mode Picker ---
# We start at the mode picker. Show it briefly.
Sleep 1.5s

# --- ECS: drill into cluster → services → tasks ---
# Select ECS
Enter
Sleep 2s

# Select the demo cluster
Enter
Sleep 2s

# We're on services. Pause to show the service list.
Sleep 1.5s

# Drill into the 'api' service
Enter
Sleep 2s

# Show tasks, pause
Sleep 1.5s

# Drill into a task
Enter
Sleep 1.5s

# Show task detail
Sleep 2s

# Go back to services
Escape
Sleep 0.5s
Escape
Sleep 0.5s
Escape
Sleep 1s

# --- Tail logs for the worker service ---
# Move down to worker
Down
Sleep 0.3s

# Tail logs with 'L'
Type "L"
Sleep 4s

# Show logs streaming in with JSON output
Sleep 3s

# Go back
Escape
Sleep 1s

# --- Switch to CloudWatch Logs ---
Type "`"
Sleep 1s

# Navigate to CloudWatch Logs
Down
Sleep 0.2s
Enter
Sleep 1.5s

# Enter a log group search
Type "/ecs/e9s-demo"
Enter
Sleep 2s

# Show log groups
Sleep 1.5s

# Go back to mode picker
Escape
Sleep 1s

# --- Switch to DynamoDB ---
Type "`"
Sleep 1s

# Find DynamoDB in the list (press 'd' to jump near it)
Down
Down
Down
Sleep 0.3s
Enter
Sleep 1.5s

# Search for tables
Type "e9s"
Enter
Sleep 2s

# Select the orders table
Enter
Sleep 2s

# Show the items table with data
Sleep 2s

# Drill into an item
Enter
Sleep 2s

# Show item detail with fields
Sleep 2s

# Go back
Escape
Sleep 0.5s
Escape
Sleep 1s

# --- Switch to SQS ---
Type "`"
Sleep 1s

# Find SQS
Down
Down
Down
Down
Down
Down
Down
Down
Sleep 0.3s
Enter
Sleep 1.5s

# Search queues
Type "e9s"
Enter
Sleep 2s

# Show queue list
Sleep 1.5s

# Select the tasks queue
Enter
Sleep 2s

# Show queue stats
Sleep 2s

# Go back to mode picker
Escape
Sleep 0.5s
Escape
Sleep 0.5s
Escape
Sleep 1s

# --- Switch to CloudWatch Alarms ---
Type "`"
Sleep 1s

# Navigate to CW Alarms
Down
Sleep 0.2s
Enter
Sleep 1.5s

# Pick "All alarms"
Enter
Sleep 2s

# Show the alarm list with colored states
Sleep 2.5s

# Drill into an alarm
Enter
Sleep 2s

# Show alarm detail
Sleep 2s

# --- Quit ---
Type "q"
Sleep 1s
