---
name: validate
description: validate the MCP tools
---

# Usage

Use this skill to validate that existing MCP tools conform to the best-practices

# Checklist 

## Tool naming

Verify that the tools defined in the `internal/argocd` package respect the following naming scheme: 

- Use snake_case with service prefix
- Format: {service}_{action}_{resource}
- Example: slack_send_message, github_create_issue