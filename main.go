package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"pedersandvoll/spt-playlist-randomizer/auth"
	"strconv"
	"strings"
	"sync"
	"time"
)

type GetUsersPlaylists struct {
	Items []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
}

func getPlaylists(client *http.Client, token string) (GetUsersPlaylists, error) {
	bearerToken := "Bearer " + token
	url := "https://api.spotify.com/v1/me/playlists"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fmt.Println(err)
		return GetUsersPlaylists{}, err
	}
	req.Header.Add("Authorization", bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return GetUsersPlaylists{}, err
	}
	defer resp.Body.Close()

	var playlistResponse GetUsersPlaylists
	if err := json.NewDecoder(resp.Body).Decode(&playlistResponse); err != nil {
		return GetUsersPlaylists{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return playlistResponse, nil
}

type GetPlaylistItems struct {
	Total int `json:"total"`
}

func getPlaylistItems(playlistId string, client *http.Client, token string) (int, error) {
	bearerToken := "Bearer " + token
	url := fmt.Sprintf("https://api.spotify.com/v1/playlists/%s/tracks", playlistId)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Add("Authorization", bearerToken)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var playlistItems GetPlaylistItems
	if err := json.NewDecoder(resp.Body).Decode(&playlistItems); err != nil {
		fmt.Println("failed to decode response: %w", err)
	}

	return playlistItems.Total, nil
}

func shufflePlaylist(playlistId string, range_start int, insert_before int, client *http.Client, token string, ch chan<- string, track_num int, playlist_tracks int) {
	bearerToken := "Bearer " + token
	data := fmt.Sprintf(`{"insert_before":%d, "range_start":%d}`, insert_before, range_start)
	url := fmt.Sprintf("https://api.spotify.com/v1/playlists/%s/tracks", playlistId)
	req, err := http.NewRequest("PUT", url, strings.NewReader(data))
	if err != nil {
		ch <- fmt.Sprintf("Error creating request for index %d: %v", range_start, err)
		return
	}
	req.Header.Add("Authorization", bearerToken)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		ch <- fmt.Sprintf("Error shuffling index %d: %v", range_start, err)
		return
	}
	defer resp.Body.Close()

	percent_done := (float64(track_num) / float64(playlist_tracks)) * 100

	ch <- fmt.Sprintf("%.2f", percent_done)
}

func main() {
	client := &http.Client{}

	token, err := auth.GetSpotifyUserAuth()
	if err != nil {
		fmt.Println("Error getting access token:", err)
		return
	}

	playlists, err := getPlaylists(client, token)
	if err != nil {
		fmt.Println(err)
	}

	itemNum := len(playlists.Items)

	fmt.Println("---------------------")

	for index, element := range playlists.Items {
		fmt.Printf("%d: %s\n", index+1, element.Name)
	}

	fmt.Println("---------------------")
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Type number of playlist you want to shuffle!")

	for scanner.Scan() {
		text := scanner.Text()
		number, err := strconv.Atoi(text)
		if err != nil {
			fmt.Println("Input is not a number")
			continue
		}

		if number >= 1 && number <= itemNum {
			timer := time.Now()
			selectedPlaylist := playlists.Items[number-1]
			fmt.Printf("You selected: %s\n", selectedPlaylist.Name)

			totalTracks, err := getPlaylistItems(selectedPlaylist.ID, client, token)
			if err != nil {
				fmt.Println("Error fetching playlist items:", err)
				return
			}
			if totalTracks == 0 {
				fmt.Println("No tracks found in this playlist.")
				return
			}

			ch := make(chan string, totalTracks)
			var wg sync.WaitGroup

			rateLimiter := time.Tick(time.Minute / 160)

			wg.Add(totalTracks)
			for i, idx := range rand.Perm(totalTracks) {
				track_num := i + 1
				go func(idx int, track_num int) {
					defer wg.Done()
					<-rateLimiter
					shufflePlaylist(selectedPlaylist.ID, (idx+1)%totalTracks, 0, client, token, ch, track_num, totalTracks)
				}(idx, track_num)
			}

			go func() {
				wg.Wait()
				close(ch)
			}()

			for msg := range ch {
				fmt.Println(msg)
			}

			fmt.Println("Shuffling took:", time.Since(timer))

			break
		}

		fmt.Println("Not valid playlist number")
	}
}
