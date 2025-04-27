package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/patrickmn/go-cache"
	"github.com/urfave/cli/v2"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

// lets make this a good go script to mak eall of this work for me
// lets make this a good go script to mak eall of this work for me 
// ChannelInfo stores information about a YouTube channel
type ChannelInfo struct {
	ID              string
	Title           string
	Description     string
	SubscriberCount string
	VideoCount      string
	ViewCount       string
	Thumbnail       string
	Score           int    // Similarity score for ranking
	TopCategories   []string // Top content categories
	ContentTags     []string // Common content tags
}

// VideoInfo stores basic information about a video
type VideoInfo struct {
	ID       string
	Title    string
	Tags     []string
	Category string
}

// ResultCache is used for caching results to improve performance
var resultCache *cache.Cache

// Regular expressions for extracting channel ID from different URL formats
var (
	channelIDRegex     = regexp.MustCompile(`(?:youtube\.com/channel/|youtube\.com/c/|youtube\.com/@)([^/?&]+)`)
	videoURLRegex      = regexp.MustCompile(`youtube\.com/watch\?v=([^&]+)`)
	customChannelRegex = regexp.MustCompile(`youtube\.com/user/([^/?&]+)`)
)

// extractChannelID tries to extract a channel ID from various formats of YouTube URLs
func extractChannelID(input string, youtubeService *youtube.Service) (string, error) {
	input = strings.TrimSpace(input)

	// If it looks like a direct channel ID
	if !strings.Contains(input, "/") && !strings.Contains(input, " ") {
		return input, nil
	}

	// Check for channel URL patterns
	match := channelIDRegex.FindStringSubmatch(input)
	if match != nil && len(match) >= 2 {
		if strings.HasPrefix(input, "youtube.com/@") {
			// Handle @username format
			call := youtubeService.Channels.List([]string{"id"}).ForUsername(match[1])
			response, err := call.Do()
			if err != nil {
				return "", err
			}
			if len(response.Items) > 0 {
				return response.Items[0].Id, nil
			}
			// If not found by username, search by handle
			searchCall := youtubeService.Search.List([]string{"snippet"}).Q(match[1]).Type("channel").MaxResults(1)
			searchResponse, err := searchCall.Do()
			if err != nil {
				return "", err
			}
			if len(searchResponse.Items) > 0 {
				return searchResponse.Items[0].Snippet.ChannelId, nil
			}
		} else {
			// Regular channel URL
			return match[1], nil
		}
	}

	// Check for video URL pattern and extract channel from video
	match = videoURLRegex.FindStringSubmatch(input)
	if match != nil && len(match) >= 2 {
		videoID := match[1]
		videoCall := youtubeService.Videos.List([]string{"snippet"}).Id(videoID)
		videoResponse, err := videoCall.Do()
		if err != nil {
			return "", err
		}
		if len(videoResponse.Items) > 0 {
			return videoResponse.Items[0].Snippet.ChannelId, nil
		}
	}

	// Check for custom channel URL format
	match = customChannelRegex.FindStringSubmatch(input)
	if match != nil && len(match) >= 2 {
		username := match[1]
		call := youtubeService.Channels.List([]string{"id"}).ForUsername(username)
		response, err := call.Do()
		if err != nil {
			return "", err
		}
		if len(response.Items) > 0 {
			return response.Items[0].Id, nil
		}
	}

	// Last resort: Search for channel by name
	searchCall := youtubeService.Search.List([]string{"snippet"}).Q(input).Type("channel").MaxResults(1)
	searchResponse, err := searchCall.Do()
	if err != nil {
		return "", err
	}
	if len(searchResponse.Items) > 0 {
		return searchResponse.Items[0].Snippet.ChannelId, nil
	}

	return "", fmt.Errorf("could not find a channel matching '%s'", input)
}

// getChannelInfo retrieves detailed information about a channel
func getChannelInfo(youtubeService *youtube.Service, channelID string) (*ChannelInfo, error) {
	call := youtubeService.Channels.List([]string{"snippet", "statistics"}).Id(channelID)
	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	if len(response.Items) == 0 {
		return nil, fmt.Errorf("channel not found: %s", channelID)
	}

	channel := response.Items[0]
	info := &ChannelInfo{
		ID:              channel.Id,
		Title:           channel.Snippet.Title,
		Description:     channel.Snippet.Description,
		SubscriberCount: formatCount(fmt.Sprintf("%d", channel.Statistics.SubscriberCount)),
		VideoCount:      formatCount(fmt.Sprintf("%d", channel.Statistics.VideoCount)),
		ViewCount:       formatCount(fmt.Sprintf("%d", channel.Statistics.ViewCount)),
		Thumbnail:       channel.Snippet.Thumbnails.High.Url,
	}

	return info, nil
}

// formatCount formats large numbers with K, M, B suffixes
func formatCount(countStr string) string {
	if countStr == "" {
		return "Hidden"
	}

	var count int64
	fmt.Sscanf(countStr, "%d", &count)

	if count >= 1000000000 {
		return fmt.Sprintf("%.1fB", float64(count)/1000000000)
	} else if count >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(count)/1000000)
	} else if count >= 1000 {
		return fmt.Sprintf("%.1fK", float64(count)/1000)
	}

	return countStr
}

// getChannelTopVideos retrieves the most popular videos from a channel
func getChannelTopVideos(youtubeService *youtube.Service, channelID string, maxResults int) ([]VideoInfo, error) {
	call := youtubeService.Search.List([]string{"snippet"}).
		ChannelId(channelID).
		Type("video").
		Order("viewCount").
		MaxResults(int64(maxResults))

	response, err := call.Do()
	if err != nil {
		return nil, err
	}

	var videos []VideoInfo
	for _, item := range response.Items {
		// Get video details including category and tags
		videoCall := youtubeService.Videos.List([]string{"snippet", "contentDetails"}).Id(item.Id.VideoId)
		videoResp, err := videoCall.Do()
		if err != nil {
			continue
		}
		
		if len(videoResp.Items) > 0 {
			videoDetails := videoResp.Items[0]
			video := VideoInfo{
				ID:       item.Id.VideoId,
				Title:    item.Snippet.Title,
				Tags:     videoDetails.Snippet.Tags,
				Category: videoDetails.Snippet.CategoryId,
			}
			videos = append(videos, video)
		}
		
		// Rate limiting
		time.Sleep(100 * time.Millisecond)
	}

	return videos, nil
}

// extractContentProfile creates a content profile based on video metadata
func extractContentProfile(videos []VideoInfo) ([]string, []string) {
	// Count categories and tags
	categoryCount := make(map[string]int)
	tagCount := make(map[string]int)
	
	for _, video := range videos {
		categoryCount[video.Category]++
		
		for _, tag := range video.Tags {
			// Normalize tags - lowercase and trim
			tag = strings.ToLower(strings.TrimSpace(tag))
			if len(tag) > 2 { // Skip very short tags
				tagCount[tag]++
			}
		}
	}
	
	// Sort categories by count
	var sortedCategories []string
	for cat := range categoryCount {
		sortedCategories = append(sortedCategories, cat)
	}
	sort.Slice(sortedCategories, func(i, j int) bool {
		return categoryCount[sortedCategories[i]] > categoryCount[sortedCategories[j]]
	})
	
	// Get top 3 categories or fewer if not available
	var topCategories []string
	for i := 0; i < len(sortedCategories) && i < 3; i++ {
		topCategories = append(topCategories, sortedCategories[i])
	}
	
	// Sort tags by count
	type tagFreq struct {
		tag   string
		count int
	}
	var tagFrequencies []tagFreq
	for tag, count := range tagCount {
		tagFrequencies = append(tagFrequencies, tagFreq{tag, count})
	}
	sort.Slice(tagFrequencies, func(i, j int) bool {
		return tagFrequencies[i].count > tagFrequencies[j].count
	})
	
	// Get top 10 tags or fewer if not available
	var topTags []string
	for i := 0; i < len(tagFrequencies) && i < 10; i++ {
		topTags = append(topTags, tagFrequencies[i].tag)
	}
	
	return topCategories, topTags
}

// findSimilarContentChannels finds channels with similar content profiles
func findSimilarContentChannels(youtubeService *youtube.Service, channelID string, maxResults int) ([]*ChannelInfo, error) {
	// Check cache first
	if cachedResults, found := resultCache.Get(channelID); found {
		return cachedResults.([]*ChannelInfo), nil
	}

	// Get source channel info
	sourceChannel, err := getChannelInfo(youtubeService, channelID)
	if err != nil {
		return nil, err
	}
	
	// Get top videos from source channel
	sourceVideos, err := getChannelTopVideos(youtubeService, channelID, 10)
	if err != nil {
		return nil, err
	}
	
	// Create content profile for source channel
	sourceCategories, sourceTags := extractContentProfile(sourceVideos)
	sourceChannel.TopCategories = sourceCategories
	sourceChannel.ContentTags = sourceTags
	
	// Prepare for collecting similar channels
	channelsMap := make(map[string]*ChannelInfo)
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Strategy 1: Search for channels using content tags
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		// Use top tags from source channel to find similar content
		for _, tag := range sourceTags {
			// Skip very generic tags
			if len(tag) < 4 {
				continue
			}
			
			call := youtubeService.Search.List([]string{"snippet"}).
				Q(tag).
				Type("channel").
				MaxResults(5)

			response, err := call.Do()
			if err != nil {
				log.Printf("Tag search error: %v", err)
				continue
			}

			for _, item := range response.Items {
				if item.Snippet.ChannelId == channelID {
					continue // Skip the source channel
				}

				mutex.Lock()
				if _, exists := channelsMap[item.Snippet.ChannelId]; !exists {
					// Get detailed info
					info, err := getChannelInfo(youtubeService, item.Snippet.ChannelId)
					if err == nil {
						// Add to map for further analysis
						channelsMap[item.Snippet.ChannelId] = info
					}
				}
				mutex.Unlock()

				// Rate limiting
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	
	// Strategy 2: Find channels making similar videos
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		// Extract titles from source videos
		var videoTitles []string
		for _, video := range sourceVideos {
			titles := strings.Split(video.Title, " ")
			for _, title := range titles {
				if len(title) > 4 { // Skip short words
					videoTitles = append(videoTitles, strings.ToLower(title))
				}
			}
		}
		
		// Use at most 3 title keywords for search
		searchLimit := 3
		if len(videoTitles) > searchLimit {
			videoTitles = videoTitles[:searchLimit]
		}
		
		for _, titleWord := range videoTitles {
			call := youtubeService.Search.List([]string{"snippet"}).
				Q(titleWord).
				Type("video").
				MaxResults(10)

			response, err := call.Do()
			if err != nil {
				log.Printf("Video search error: %v", err)
				continue
			}

			for _, item := range response.Items {
				if item.Snippet.ChannelId == channelID {
					continue // Skip the source channel
				}

				mutex.Lock()
				if _, exists := channelsMap[item.Snippet.ChannelId]; !exists {
					// Get detailed info
					info, err := getChannelInfo(youtubeService, item.Snippet.ChannelId)
					if err == nil {
						// Add to map for further analysis
						channelsMap[item.Snippet.ChannelId] = info
					}
				}
				mutex.Unlock()

				// Rate limiting
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()
	
	// Wait for all go routines to finish
	wg.Wait()
	
	// Now analyze and score each potential similar channel
	var channels []*ChannelInfo
	for id, channelInfo := range channelsMap {
		// Get videos for this potential match
		videos, err := getChannelTopVideos(youtubeService, id, 5)
		if err != nil {
			continue
		}
		
		// Extract content profile
		categories, tags := extractContentProfile(videos)
		channelInfo.TopCategories = categories
		channelInfo.ContentTags = tags
		
		// Calculate similarity score based on content overlap
		channelInfo.Score = calculateContentSimilarity(sourceChannel, channelInfo)
		
		// Add to results if the score is above threshold
		if channelInfo.Score > 0 {
			channels = append(channels, channelInfo)
		}
		
		// Rate limiting
		time.Sleep(200 * time.Millisecond)
	}

	// Sort channels by content similarity score
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Score > channels[j].Score
	})

	// Cache results for future use
	resultCache.Set(channelID, channels, cache.DefaultExpiration)

	// Return top N channels
	if len(channels) > maxResults {
		return channels[:maxResults], nil
	}
	return channels, nil
}

// calculateContentSimilarity scores similarity between two channels based on content
func calculateContentSimilarity(source, target *ChannelInfo) int {
	score := 0
	
	// Compare category overlap
	categoryOverlap := 0
	for _, sourceCat := range source.TopCategories {
		for _, targetCat := range target.TopCategories {
			if sourceCat == targetCat {
				categoryOverlap++
			}
		}
	}
	// Award points based on category match (category matching is important)
	score += categoryOverlap * 5
	
	// Compare tag overlap
	tagOverlap := 0
	for _, sourceTag := range source.ContentTags {
		for _, targetTag := range target.ContentTags {
			// Compare cleaned tags
			sourceClean := strings.ToLower(strings.TrimSpace(sourceTag))
			targetClean := strings.ToLower(strings.TrimSpace(targetTag))
			
			if sourceClean == targetClean {
				tagOverlap++
			}
		}
	}
	// Award points based on tag matches
	score += tagOverlap * 2
	
	// Bonus for channel size similarity (can indicate similar content niche)
	var sourceSubCount, targetSubCount int64
	fmt.Sscanf(strings.TrimSuffix(source.SubscriberCount, "K"), "%f", &sourceSubCount)
	fmt.Sscanf(strings.TrimSuffix(target.SubscriberCount, "K"), "%f", &targetSubCount)
	
	// If sizes are within same magnitude, add small bonus
	subRatio := float64(sourceSubCount) / float64(targetSubCount)
	if subRatio >= 0.1 && subRatio <= 10 {
		score++
	}
	
	return score
}

func main() {
	// Initialize cache
	resultCache = cache.New(5*time.Minute, 10*time.Minute)

	// Initialize the YouTube API service
	ctx := context.Background()
	youtubeService, err := youtube.NewService(ctx, option.WithAPIKey(""))
	if err != nil {
		log.Fatalf("Error creating YouTube service: %v", err)
	}

	app := &cli.App{
		Name:  "YouTube Content Similarity Finder",
		Usage: "Find YouTube channels with similar content based on channel ID or URL",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "channel",
				Usage:    "YouTube channel URL or ID",
				Required: true,
			},
		},
		Action: func(c *cli.Context) error {
			channelURL := c.String("channel")
			channelID, err := extractChannelID(channelURL, youtubeService)
			if err != nil {
				return fmt.Errorf("could not extract channel ID: %v", err)
			}

			fmt.Println("Finding channels with similar content...")
			fmt.Println("This may take a minute as we analyze content profiles...")
			
			// Get similar content channels
			channels, err := findSimilarContentChannels(youtubeService, channelID, 10)
			if err != nil {
				return fmt.Errorf("error finding similar channels: %v", err)
			}

			// Display the results
			if len(channels) == 0 {
				fmt.Println("No similar channels found. Try a different channel or adjust search parameters.")
				return nil
			}
			
			fmt.Printf("\nFound %d channels with similar content:\n\n", len(channels))
			for i, channel := range channels {
				fmt.Printf("%d. %s (%s)\n", i+1, color.CyanString(channel.Title), channel.ID)
				fmt.Printf("   Subscribers: %s | Videos: %s | Views: %s\n", color.GreenString(channel.SubscriberCount), color.GreenString(channel.VideoCount), color.GreenString(channel.ViewCount))
				
				// Show content similarity factors
				fmt.Print("   Content topics: ")
				for j, category := range channel.TopCategories {
					if j > 0 {
						fmt.Print(", ")
					}
					fmt.Print(color.YellowString(category))
				}
				fmt.Println()
				
				fmt.Printf("   Common tags: %s\n", strings.Join(channel.ContentTags[:min(5, len(channel.ContentTags))], ", "))
				fmt.Printf("   Similarity score: %d\n", channel.Score)
				fmt.Println("   Description: " + truncateString(channel.Description, 100))
				fmt.Println()
			}

			return nil
		},
	}

	// Start the application
	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
