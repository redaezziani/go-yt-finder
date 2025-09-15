# YouTube Content Similarity Finder

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://golang.org/)
[![YouTube API](https://img.shields.io/badge/YouTube%20API-v3-FF0000?style=for-the-badge&logo=youtube&logoColor=white)](https://developers.google.com/youtube/v3)
[![CLI](https://img.shields.io/badge/CLI-Tool-4285F4?style=for-the-badge&logo=terminal&logoColor=white)](https://github.com/urfave/cli)
[![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)](LICENSE)

![GitHub issues](https://img.shields.io/github/issues/redaezziani/go-yt-finder?style=flat-square)
![GitHub stars](https://img.shields.io/github/stars/redaezziani/go-yt-finder?style=flat-square)
![GitHub forks](https://img.shields.io/github/forks/redaezziani/go-yt-finder?style=flat-square)
![Go Report Card](https://goreportcard.com/badge/github.com/redaezziani/go-yt-finder)

## 🎯 Overview

YouTube Content Similarity Finder is a powerful Go CLI application that discovers YouTube channels with similar content based on advanced content analysis. It analyzes video metadata, tags, categories, and content patterns to find channels that produce similar content to any given channel.

## ✨ Features

- 🔍 **Smart Channel Discovery**: Finds similar channels using multiple search strategies
- 🏷️ **Content Analysis**: Analyzes video tags, categories, and metadata for similarity scoring
- ⚡ **Concurrent Processing**: Utilizes goroutines for fast parallel processing
- 💾 **Intelligent Caching**: Built-in caching system to improve performance
- 🎨 **Beautiful CLI Output**: Colored and formatted terminal output
- 🔗 **Flexible Input**: Accepts various YouTube URL formats and channel IDs
- 📊 **Similarity Scoring**: Advanced algorithm to rank channels by content similarity

## 🛠️ Tech Stack

- **Language**: Go 1.21+
- **YouTube API**: Google YouTube Data API v3
- **CLI Framework**: urfave/cli/v2
- **Caching**: patrickmn/go-cache
- **Colors**: fatih/color
- **Concurrency**: Native Go goroutines and sync primitives

## 📦 Dependencies

```go
github.com/fatih/color           // Terminal colors
github.com/patrickmn/go-cache    // In-memory caching
github.com/urfave/cli/v2         // CLI framework
google.golang.org/api/youtube/v3 // YouTube API client
