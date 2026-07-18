//go:build ignore

package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func main() {
	query := `{"query":"query($search:String,$genre:String,$page:Int,$perPage:Int){Page(page:$page,perPage:$perPage){media(search:$search,genre:$genre,type:ANIME){id title{romaji english}coverImage{large}averageScore format episodes status}}}","variables":{"search":"Naruto","genre":null,"page":1,"perPage":5}}`
	resp, err := http.Post("https://graphql.anilist.co", "application/json", strings.NewReader(query))
	if err != nil {
		fmt.Println("ERROR:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	if len(body) > 500 {
		fmt.Printf("Body: %s\n", string(body)[:500])
	} else {
		fmt.Printf("Body: %s\n", string(body))
	}
}
