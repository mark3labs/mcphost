#!/usr/bin/env -S mcphost script
---
# Example script demonstrating both environment variable and script argument substitution
# Environment variables are processed first, then script arguments

mcpServers:
  github:
    type: local
    command: ["gh", "api"]
    environment:
      GITHUB_TOKEN: "${env://GITHUB_TOKEN}"
      DEBUG: "${env://DEBUG:-false}"

  filesystem:
    type: builtin
    name: fs
    options:
      allowed_directories: ["${env://WORK_DIR:-/tmp}"]

model: "${env://MODEL:-anthropic:claude-sonnet-4-20250514}"
debug: ${env://DEBUG:-false}
---
List ${repo_type:-public} repositories for user ${username}.
Use the GitHub API to fetch ${count:-10} repositories.
Working directory is ${env://WORK_DIR:-/tmp}.

# Usage:
# 1. Set environment variables:
#    export GITHUB_TOKEN="ghp_your_token_here"
#    export DEBUG="true"
#    export WORK_DIR="/home/user/projects"
#
# 2. Run with script arguments:
#    mcphost script env-substitution-script.sh --args:username alice --args:repo_type private --args:count 5
#
# This will:
# - Use GITHUB_TOKEN from environment
# - Set DEBUG=true from environment
# - Use WORK_DIR=/home/user/projects from environment
# - Use username=alice from script args
# - Use repo_type=private from script args
# - Use count=5 from script args