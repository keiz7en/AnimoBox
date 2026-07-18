//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	client := &http.Client{Timeout: 15 * time.Second}

	// Test 1: Text search
	fmt.Println("=== TEST 1: Text search 'Naruto' ===")
	searchQuery := `{"query":"query($search:String,$page:Int,$perPage:Int){Page(page:$page,perPage:$perPage){media(search:$search,type:ANIME){id title{romaji english}}}}","variables":{"search":"Naruto","page":1,"perPage":3}}`
	testRequest(client, searchQuery)

	// Test 2: Genre search
	fmt.Println("\n=== TEST 2: Genre search 'Action' ===")
	genreQ := `{"query":"query($genre:String,$page:Int,$perPage:Int){Page(page:$page,perPage:$perPage){media(genre:$genre,type:ANIME,sort:POPULARITY_DESC){id title{romaji english}}}}","variables":{"genre":"Action","page":1,"perPage":3}}`
	testRequest(client, genreQ)
}

func testRequest(client *http.Client, body string) {
	resp, err := client.Post("https://graphql.anilist.co", "application/json", bytes.NewReader([]byte(body)))
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	var pretty map[string]interface{}
	json.Unmarshal(b, &pretty)
	out, _ := json.MarshalIndent(pretty, "", "  ")
	fmt.Printf("%s\n", string(out)[:min(800, len(out))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
