package main

import (
	"bytes"
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
	http.HandleFunc("/subscribe", handleSubscription)
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

func handleSubscription(w http.ResponseWriter, r *http.Request) {
	type AuraqHandleSubscriptionRequest struct {
		Email string `json:"email"`
	}
	var subscriptionRequest AuraqHandleSubscriptionRequest
	err := json.NewDecoder(r.Body).Decode(&subscriptionRequest)
	if err != nil {
		log.Fatalf(err.Error())
	}
			
	type AirtableCreateSubscriberRequest struct {
		Records []struct {
			Fields struct {
				Email string `json:"email"`
			} `json:"fields"`
		} `json:"records"`
	}
	var airtableCreateSubscriberRequest AirtableCreateSubscriberRequest
	airtableCreateSubscriberRequest.Records = append(airtableCreateSubscriberRequest.Records, struct {
		Fields struct {
			Email string `json:"email"`
		} `json:"fields"`
	}{Fields: struct {
		Email string `json:"email"`
	}{Email: subscriptionRequest.Email}})
	
	
	
	airtableCreateSubscriberRequestObj, requestParseError := json.Marshal(airtableCreateSubscriberRequest)
	if requestParseError != nil {
		log.Fatalf(requestParseError.Error())
	}
	

	path := fmt.Sprintf("%s/%s/%s", os.Getenv("AIRTABLE_API_URL"), os.Getenv("AIRTABLE_BASE"), "Subscribers")	

	request, requestError := http.NewRequest("POST", path, bytes.NewBuffer(airtableCreateSubscriberRequestObj))

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

	fmt.Fprint(w, response.Status)
}

type AirtablePics []struct {
	Url string `json:"url"`
}

func handleSlackIntegration(w http.ResponseWriter, r *http.Request) {
	type SlackBotEventNotification struct {
		Event struct {
			Text string `json:"text"`
			Challenge string `json:"challenge"`
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

	if(len(postId) != 0) {

		airtablePicRecords := make(chan AirtablePics)
		go retrievePost(postId, airtablePicRecords)
	
		createdPicsCount := make(chan int)	
		airtablePics := <- airtablePicRecords
		go createPostImageDirectory(postId, airtablePics, createdPicsCount)
		
		if <-createdPicsCount == len(airtablePics) {
		// fmt.Fprint(w, triggerDeploy(postId))
		fmt.Fprint(w, "Success")
		}
	}
	fmt.Fprint(w, slackBotEventNotification.Event.Challenge)
	
		
}

func retrievePost(id string, res chan AirtablePics ) {
	type AirtableRetrievePostResponse struct {		
		ID string `json:"id"`
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
		Trigger_Title string `json:"trigger_title"`
	}

	var netlifyDeploy NetlifyDeploy = NetlifyDeploy{
		Trigger_Branch: "master",
		Trigger_Title: fmt.Sprintf("Deploying %s from Airtable via Slackbot", postId),
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
