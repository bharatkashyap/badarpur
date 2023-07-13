package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatal("Error loading .env file")
	}
	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/health-check", handleHealthCheck)
	http.HandleFunc("/subscribers", handleSubscription)
	http.HandleFunc("/slack", handleSlackIntegration)
	log.Fatal(http.ListenAndServe(":8000", nil))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	html, err := os.ReadFile("./static/index.html")
	if err != nil {
		fmt.Print(err)
	}
	htmlString := string(html)
	fmt.Fprint(w, htmlString)
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	html := `<html><body style="font-family: sans-serif"><h1>OK</h1></body></html>`
	fmt.Fprint(w, html)
}

// List of allowed origins
var allowedOrigins = []string{
	"https://www.auraq.in",
	"https://auraq.in",
	"http://localhost:8080",
}

func handleSubscription(w http.ResponseWriter, r *http.Request) {

	// Get the value of the "Origin" header
	origin := r.Header.Get("Origin")
	log.Default().Println(origin)
	// Set CORS headers for the preflight request
	// Allows requests from origin https://www.auraq.in with Authorization header
	// Check if the origin is present in the allowed origins whitelist
	for _, allowedOrigin := range allowedOrigins {
		if origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
	}

	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-(*A)llow-Methods", "POST")
	w.Header().Set("Access-Control-All(*o)w-Credentials", "true")

	if r.Method == "POST" {
		type AuraqHandleSubscriptionRequest struct {
			Email string `json:"email"`
		}

		var subscriptionRequest AuraqHandleSubscriptionRequest
		err := json.NewDecoder(r.Body).Decode(&subscriptionRequest)
		if err != nil {
			log.Fatalf(err.Error())
		}

		/*
			email	Request body	String	Required	The email address of the new susbcriber.
			name	Request body	String	Required	The name of the new subscriber.
			status	Request body	String	Required	The status of the new subscriber. Can be enabled, disabled or blocklisted.
			lists	Request body	Numbers	Optional	Array of list IDs to subscribe to (marked as unconfirmed by default).
			attribs	Request body	json	Optional	JSON list containing new subscriber's attributes.
			preconfirm_subscriptions	Request body	Bool	Optional	If true, marks subscriptsions as confirmed and no-optin e-mails are sent for double opt-in lists.
		*/

		type ListmonkCreateSubscriberRequest struct {
			Email  string `json:"email"`
			Name   string `json:"name"`
			Status string `json:"status"`
			Lists  []int  `json:"lists"`
		}

		var listmonkCreateSubscriberRequest ListmonkCreateSubscriberRequest
		listmonkCreateSubscriberRequest.Email = subscriptionRequest.Email
		listmonkCreateSubscriberRequest.Name = subscriptionRequest.Email
		listmonkCreateSubscriberRequest.Status = "enabled"
		listmonkCreateSubscriberRequest.Lists = []int{2}

		listmonkCreateSubscriberRequestObj, requestParseError := json.Marshal(listmonkCreateSubscriberRequest)

		if requestParseError != nil {
			log.Fatalf(requestParseError.Error())
		}
		path := fmt.Sprintf("%s/%s", os.Getenv("LISTMONK_API_URL"), "subscribers")

		request, requestError := http.NewRequest("POST", path, bytes.NewBuffer(listmonkCreateSubscriberRequestObj))

		if requestError != nil {
			log.Fatalf(requestError.Error())
		}

		request.Header.Set("Content-Type", "application/json; charset=UTF-8")
		request.Header.Set("Authorization", fmt.Sprintf("Basic %s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", os.Getenv("LISTMONK_USERNAME"), os.Getenv("LISTMONK_PASSWORD"))))))

		client := &http.Client{}
		response, err := client.Do(request)
		if err != nil {
			log.Fatalf(err.Error())
		}

		fmt.Fprint(w, response.Status)
	}

}

type AirtablePics []struct {
	Url string `json:"url"`
}

func handleSlackIntegration(w http.ResponseWriter, r *http.Request) {
	type SlackBotEventNotification struct {
		Challenge string `json:"challenge"`
		Event     struct {
			Text string `json:"text"`
		} `json:"event"`
	}

	var slackBotEventNotification SlackBotEventNotification
	err := json.NewDecoder(r.Body).Decode(&slackBotEventNotification)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	regex, _ := regexp.Compile("rec.*")
	postId := regex.FindString(slackBotEventNotification.Event.Text)

	if len(postId) != 0 {

		airtablePicRecords := make(chan AirtablePics)
		go retrievePost(postId, airtablePicRecords)

		createdPicsCount := make(chan int)
		airtablePics := <-airtablePicRecords
		go createPostImageDirectory(postId, airtablePics, createdPicsCount)

		if <-createdPicsCount == len(airtablePics) {
			fmt.Fprint(w, triggerDeploy(postId))
		}
	}
	fmt.Fprint(w, slackBotEventNotification.Challenge)
}

func retrievePost(id string, res chan AirtablePics) {
	type AirtableRetrievePostResponse struct {
		ID     string `json:"id"`
		Fields struct {
			Pics AirtablePics `json:"Pics"`
		} `json:"fields"`
	}

	path := fmt.Sprintf("%s/%s/%s/%s", os.Getenv("AIRTABLE_API_URL"), os.Getenv("AIRTABLE_BASE"), "Posts", id)
	request, requestError := http.NewRequest("GET", path, nil)

	if requestError != nil {
		log.Fatalf(requestError.Error())
	}

	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Set("Authorization", fmt.Sprintf("Bearer %s", os.Getenv("AIRTABLE_API_KEY")))
	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Fatalf(err.Error())
	}

	var airtableRetrievePostResponse AirtableRetrievePostResponse
	responseParseError := json.NewDecoder(response.Body).Decode(&airtableRetrievePostResponse)

	if responseParseError != nil {
		log.Fatalf(responseParseError.Error())
	}
	res <- airtableRetrievePostResponse.Fields.Pics
	defer response.Body.Close()
}

func downloadPic(url string, dir string, name string, res chan os.File) {
	response, e := http.Get(url)
	if e != nil {
		log.Fatal(e)
	}
	defer response.Body.Close()
	path := filepath.Join(dir, fmt.Sprintf("%s.jpg", name))
	file, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}

	_, err = io.Copy(file, response.Body)
	if err != nil {
		log.Fatal(err)
	}
	res <- *file
	defer file.Close()
}

func createPostImageDirectory(postId string, airtablePics AirtablePics, res chan int) {
	path := filepath.Join("static", "auraq", postId)
	os.RemoveAll(path)
	os.Mkdir(path, 0755)
	for index, pic := range airtablePics {
		file := make(chan os.File)
		go downloadPic(pic.Url, path, fmt.Sprint(index), file)
		fmt.Print(<-file)
	}
	files, _ := ioutil.ReadDir(path)
	res <- len(files)
}

func triggerDeploy(postId string) string {
	type NetlifyDeploy struct {
		Trigger_Branch string `json:"trigger_branch"`
		Trigger_Title  string `json:"trigger_title"`
	}

	var netlifyDeploy NetlifyDeploy = NetlifyDeploy{
		Trigger_Branch: "master",
		Trigger_Title:  fmt.Sprintf("Deploying %s from Airtable via Slackbot", postId),
	}

	netlifyDeployObj, requestParseError := json.Marshal(netlifyDeploy)
	if requestParseError != nil {
		log.Fatalf(requestParseError.Error())
	}

	path := fmt.Sprintf("%s/%s/%s", os.Getenv("NETLIFY_API_URL"), "build_hooks", os.Getenv("NETLIFY_BUILD_HOOK_TOKEN"))
	fmt.Print(path)
	request, requestError := http.NewRequest("POST", path, bytes.NewBuffer(netlifyDeployObj))

	if requestError != nil {
		log.Fatalf(requestError.Error())
	}

	request.Header.Set("Content-Type", "application/json; charset=UTF-8")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		log.Fatalf(err.Error())
	}

	return response.Status
}
