# import_dropbox_into_outline

A Go tool to import Markdown files into Outline Wiki while preserving folder structure and hierarchical relationships.

## Overview

This tool takes a directory of Markdown files (such as those exported from Dropbox Paper) and imports them into an Outline Wiki instance. It maintains the folder hierarchy by creating document structures in Outline that mirror the original folder organization.

## Prerequisites

- Go 1.23 or higher
- Access to an Outline Wiki instance
- Outline API token with appropriate permissions
- Collection of Markdown files to import

## Setup

1. Obtain your Outline API token:
- Log into your Outline instance
- Go to Settings → API Tokens
- Create a new token with appropriate permissions

2. Identify your target collection:
- Use the `-list` flag to view available collections
- Note the UUID of the collection where you want to import documents

## Usage

The tool supports several command-line flags:

```bash
go run main.go [flags]

Flags:
-folder string
Folder containing Markdown files (default "output_paper_markdown")
-host string
Outline host URL (default "https://app.getoutline.com")
-collection string
Valid collection UUID to import documents into
-token string
Outline API token (or use OUTLINE_API_TOKEN env var)
-list
List collections and exit
-debug
Enable debug logging
```

Example usage:

```bash
# List available collections
go run main.go -token "your_token" -list

# Import documents
go run main.go -token "your_token" -collection "uuid" -folder "./markdown_files"
```

## Features

- Preserves folder hierarchy by creating parent-child document relationships
- Supports nested folders of any depth
- Maintains document organization through Outline's document hierarchy
- Provides detailed logging and error reporting
- Creates folder documents automatically as needed

## File Organization

The tool processes your Markdown files as follows:
- Each folder becomes a document in Outline
- Files within folders become child documents
- The hierarchy is preserved through Outline's parent-child document relationships

## Error Handling

The tool includes comprehensive error handling:
- Continues processing even if individual files fail
- Provides detailed error messages
- Supports debug mode for verbose logging
- Validates API token and collection ID before starting

## Limitations

- Only processes `.md` files
- Requires appropriate Outline API permissions
- Some Markdown formatting may need adjustment based on Outline's supported syntax