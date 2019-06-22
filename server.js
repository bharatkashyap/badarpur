const express = require('express');
const airtable = require('airtable');
const dotenv = require('dotenv');

if(process.env.NODE_ENV !== "Production") {
    dotenv.config();
}

const app = express();
app.use(express.static("img"));
let port = process.env.PORT || 3000;

const airtableAPIKey = process.env.AIRTABLE_API_KEY;
const airtableBaseId = process.env.AIRTABLE_BASE_ID;
const base = new airtable({apiKey: airtableAPIKey}).base(airtableBaseId);
const airtableConfig = {
    places: "Places",
    comments: "Comments"
}

app.get("/", index);
app.get("/comments", fetchComments);
app.get("/posts", fetchPosts);

const indexComponent = require('./index');
function index(req, res) {
    res.send(indexComponent.template);
}

function fetchPosts(req, res) {
    
    base(airtableConfig.places).select({
        sort: [{
            field: "Date", 
            direction: "desc"
        }]
    }).eachPage(function page(records, fetchNextPage) {
        // This function (`page`) will get called for each page of records.
        res.header('Access-Control-Allow-Origin', '*');
        res.send(records);
        
    
        // To fetch the next page of records, call `fetchNextPage`.
        // If there are more records, `page` will get called again.
        // If there are no more records, `done` will get called.
        fetchNextPage();
    
    }, function done(err) {
        if (err) { console.error(err); return; }
    });
}

function fetchComments() {

}


app.listen(port, console.log("Backend running on port", port));