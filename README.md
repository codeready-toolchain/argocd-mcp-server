# Argo CD MCP

Argo CD MCP is a Model Context Protocol Server to converse with Argo CD from a UI such as Anthropic's Claude or Block's Goose

## Features

- Prompts:
  - `argocd-unhealthy-application-resources`: list the Unhealthy (`Degraded` and `Progressing`) Applications in Argo CD
- Tools:
  - `unhealthyApplications`: list the Unhealthy (`Degraded` and `Progressing`) Applications in Argo CD
  - `unhealthyApplicationResources`: list unhealthy resources of a given Argo CD Application

Example:

> list the unhealthy applications on Argo CD and for each one, list their unhealthy resources


## Building and Installing

Requires [Go 1.24 (or higher)](https://go.dev/doc/install) and [Task](https://taskfile.dev/)

### Testing and Linting

```bash
task test test-e2e lint
```

Note: the e2e tests rely on Podman and the podman-compose extension to build images and run containers for the MCP server and a mock instance of Argo CD. 
See [Podman installation](https://podman.io/docs/installation) and [podman-compose extension installtion](https://github.com/containers/podman-compose?tab=readme-ov-file#installation) to setup these tools in your local environment.

### Building

Build the binary using the following command:

```bash
task install
```

Build the Container image with the following command:

```bash
task build-image
```

## Using the Argo CD MCP Server

### Obtaining a token to connect to Argo CD

Create a local account in Argo CD with `apiKey` capabilities only (not need for `login`). See [Argo CD documentation for more information](https://argo-cd.readthedocs.io/en/stable/operator-manual/user-management/). 
Once create, generate a token via the 'Settings > Accounts' page in the Argo CD UI or via the `argocd account generate-token` command and store the token in a `token-file` which will be passed as an argument when running the server (see below).

### Stdio Transport with Claude Desktop App

On macOS, run the following command:

```
code ~/Library/Application\ Support/Claude/claude_desktop_config.json
```

and add the following MCP server definition:
```
{
    "mcpServers": {
        "argocd-mcp-server": {
            "command": "<path/to/argocd-mcp-server>",
            "args": [
                "--transport",
                "stdio",
                "--argocd-token"
                "<token>",
                "--argocd-url",
                "<url>",
                "--insecure",
                "<true|false>",
                "debug",
                "<true|false>"
            ]
        }
    }
}
```

### Stdio Transport in Cursor

Edit your `~/.cursor/mcp.json` file with the following contents:

```
{
  "mcpServers": {
    "argocd-mcp-server": {
      "command": "<path/to/argocd-mcp-server>",
      "args": [
        "--transport",
        "stdio",
        "--argocd-token",
        "<token>",
        "--argocd-url",
        "<url>",
        "--insecure",
        "<true|false>",
        "--debug",
        "<true|false>"
      ]
    }
  }
}
```

### HTTP Transport with Cursor

Start the Argo CD MCP server from the binary after running `task install`:

```
argocd-mcp-server --transport=http --argocd-url=<url> --argocd-token=<token> --debug=<true|false> --listen=<[host]:port>
```

Or start the Argo CD MCP server as a container after running `task build-image`:

```bash
podman run -d --name argocd-mcp-server --transport http -e ARGOCD_MCP_SERVER_LISTEN_HOST=0.0.0.0 -e ARGOCD_MCP_URL=<url> -e ARGOCD_MCP_TOKEN=<token> -e ARGOCD_MCP_DEBUG=<true|false> -p 8080:8080 argocd-mcp-server:latest
```

Edit your `~/.cursor/mcp.json` file with the following contents:

```
{
  "mcpServers": {
    "argocd-mcp-server": {
      "url": "http://localhost:8080/mcp"
    }
  }
}
```

## License

The code is available under the Apache License 2.0
