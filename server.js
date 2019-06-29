const express = require('express');
const airtable = require('airtable');
const dotenv = require('dotenv');
const bodyParser = require('body-parser');


if(process.env.NODE_ENV !== "Production") {
    dotenv.config();
}

const app = express();
app.use(express.static("img"));
let port = process.env.PORT || 3000;

const airtableAPIKey = process.env.AIRTABLE_API_KEY;
const airtableBaseId = process.env.AIRTABLE_BASE_ID;
const apiBearerToken = process.env.API_BEARER_TOKEN;
const base = new airtable({apiKey: airtableAPIKey}).base(airtableBaseId);
const airtableConfig = {
    places: "Places",
    comments: "Comments",
    users: "Users"
}

if(process.env.NODE_ENV === "Development") {
    app.use( (req, res, next) => {
        res.header("Access-Control-Allow-Origin", "*");
        res.header("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Authorization, Accept");
        next();
    })
}

app.use(bodyParser.json());

app.get("/", index);
app.get("/comments", fetchComments);
app.get("/posts", fetchPosts);
app.get("/post/:id", fetchPost);
app.post("/like", validateToken, likePost);
app.post("/user", validateToken, fetchUser);


const indexComponent = require('./index');
function index(req, res) {
    res.send(indexComponent.template);
}

function validateToken(req, res, next) {
    const authorizationHeader = req.headers.authorization;
    if(authorizationHeader) {
        const token = authorizationHeader.split(' ')[1];
        if (token === apiBearerToken) next();
        else {
            result = {
                status: 401,
                error: 'Unauthorized.'
            }
            res.status(401).send(result);
        }
    }
    else {
        result = {
            status: 403,
            error: 'Forbidden.'
        }
        res.status(403).send(result);
    }
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

function fetchPost(req, res) {
    const id = req.params.id;

    base(airtableConfig.places).find(id, (err, record) => {
        if(err) { console.error(err); return; res.status(501).send(err); }
        res.status(200).send(record);
    });
}

function fetchComments() {

}

async function findUser(user) {
    
    let foundUserFlag = false;
    let foundUser = new Promise( (resolve, reject) => {
        
        base(airtableConfig.users).select({}).eachPage(function page(records, fetchNextPage) {
            
            records.some(function(record) {
                if(record.get('email') === user.email) { foundUserFlag = true; resolve(record); }
            });
            fetchNextPage();
        }, function done(err) {
            if(err) { console.error(err); }
            if(foundUserFlag === false) { resolve(null) }
        })

    });
    
    return await foundUser;

}

async function fetchUser(req, res) {
    const user = req.body.user;
    let userExists = await findUser(user);
    
    if(userExists) { res.status(200).send(userExists); return; }
    
    let createdUser = new Promise( (resolve, reject) => {
        base(airtableConfig.users).create(user, (err, record) => {
            if(err) { console.error(err); return; }
            resolve(record);
        })
    });

    res.status(200).send(await createdUser);

}

async function likePost(req, res) {
    
    const user = req.body.user;
    const posts = req.body.posts;
    let likedPosts = new Promise( (resolve, reject) => {
        base(airtableConfig.users).update(user, {
            "Likes": posts
        }, (err, record) => {
            if(err) { console.error(err); reject(false); return; }
            resolve(record.get('Likes'));
        })
    });

    res.status(200).send(await likedPosts);
}


app.listen(port, console.log("Backend running on port", port));