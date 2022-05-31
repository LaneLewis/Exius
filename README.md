# What is Exius
At it's core, Exius implements the webdav protocol over a cloud storage provider with the ability to create scoped permissions with limited capabilities for manipulating data in order to combat bad actors. A typical use case for Exius would be the following: An a researcher wants to give human subjects the ability to upload a single csv file to a particular file in their cloud storage, but they don't want to give them access to view the entire folder or upload more than a single file or upload any other type of file than csv. So they create an Exius server and generate a unique key for each participant with only the ability to PUT a single file of type text/csv. They then pass the key to their participant who then uploads their single csv.

For a nodejs/javascript SDK to talk with an Exius server visit [exius-sdk](https://github.com/LaneLewis/exius-sdk).
For a step by step setup guide on launching an exius server visit [exius-launchers](https://github.com/LaneLewis/Exius-Launchers).
# Features
 Exius allows users to create access keys that grant permission to limited Webdav operations on specific files and folders within an administrator's cloud storage account. In addition, these keys can be given only access to limited file types for uploading, limited size upload size, and maximum numbers of uploads. If users do not want to connect to a cloud storage provider, they can instead store data directly on the server in the data folder. Key information is stored in a postgresql database. The webdav protocol and connection to a cloud storage provider is carried out using Rclone. 
# API
| location | protocol | authentication | body | function |
| -------- | -------- |--------------- | ---- | -------- |
| /addKey  | POST     | access key     | json | Creates a new key using a passed url/json body and a key in the authorization header. This new key must have lesser permissions than the key that is creating it. It returns the created key and all of its parameters. |
| /getKey  | GET      | access key     | none | Returns all parameters of the key. |
| /deleteKey | GET    | access key     | none | Deletes the key. |
| /getChildKeys | GET | access key     | none | Returns all keys with lesser permissions than the access key along with their endpoints' relative paths from the access key. |
| /files/{endpoint}/{path} | COPY, DELETE, GET, HEAD, LOCK, MKCOL, MOVE, OPTIONS, POST, PROPFIND, PUT, TRACE, UNLOCK | access key | depends | Does a webdav operation on some file or folder in the cloud storage. |
| /admin | GET | access key | None | Provides a web interface for users with root access to access their data and view their files. This is especially useful if a user is storing data on Exius and not through a cloud provider. |

## /addKey
The most important and complex of the endpoints is addKey. All parameters of the added key must be a weaker or equal to the access key in all BOOL fields. This ensures that if a created key has the ability to create more children keys, they act off of a waterfall permission structure and cannot have greater permissions than themselves. 

JSON Parameters
| JSON Field | Required | Type | Default | Description |
| --- | --- | --- | --- | --- | --- |
| /CanCreateChild | false | BOOL | false | Is the key able to create other keys with lesser or equal permisssions |
| /InitiateExpire | false | STRING (Creation,Get, Mkcol, Never,Put) | Creation | Webdav or key creation as action to start the timer for the key to expire |
| /ExpireDelta | false | POSITIVE INT64 | 3600000 | Milliseconds until the key expires from the initiation specified |
| /Endpoints/{endpoint} | true | JSON MAP | none | Parameters for each endpoint being created |

Endpoint parameters
| JSON Field | Required | Type | Default | Description |
| --- | --- | --- | --- | --- | --- |
| /Endpoints/{endpoint}/Path | true | STRING | "" | Relative folder path from access key that this key will have access to. Must be a relative path from an endpoint of the access key. |
| /Endpoints/{endpoint}/MaxMkcol | false | POSITIVE INT32 | 2147483647 | Maximum number of directories that can be created by this key on this endpoint|
| /Endpoints/{endpoint}/MaxPut | false | POSITIVE INT32 | 2147483647 | Maximum number of PUT operations that can be done by this key on this endpoint|
| /Endpoints/{endpoint}/MaxPutSize | false | POSITIVE INT64 | 9223372036854775807 | Maximum size in bytes of PUT request that can be done by this key on this endpoint| 
| /Endpoints/{endpoint}/MaxGet | false | POSITIVE INT32 | 2147483647 | Maximum number of GET operations that can be done by this key on this endpoint|
| /Endpoints/{endpoint}/PutTypes | false | ARRAY(STRING("any" or text encoding -"csv/text" - etc.)) | "any" | Enforced encoding type of all files given by PUT request to this endpoint. |
| /Endpoints/{endpoint}/{Copy, Delete, Get, Head, Lock, Mkcol, Move, Options, Post, Propfind, Put, Trace, Unlock} | false | BOOL | false | Whether the key has access to the Webdav protocol on the folder. 

So, an example json body for adding a new key from the root key with access only to PUT 1 file of type "text/plain" within a window of 1 hour to the folder "upload" in the root directory would look like: 
```json
{
    "CanCreateChild":false,
    "Endpoints":{
        "subjectCsv":{"PUT" : true, "path":"root/upload", "PutTypes":["text/plain"], "MaxPut":1},
        },
    "InitiateExpire":"Creation",
    "ExpireDelta":3600000}
}
```
# Setting Up an Exius Instance
For a step-by-step guide on how to set up an Exius server in the cloud visit [exius-launchers](https://github.com/LaneLewis/Exius-Launchers).

Exius exists as a docker container at that can be pulled at `ghcr.io/lanelewis/exius:latest`.
To connect to a cloud storage backend, Exius needs an Rclone config file with credentials copied into the container at `/root/.config/rclone/rclone.conf`. By default, Exius has an Rclone config file with the remote name 'data' pointing to the folder `app/data` inside the container.

The required env variables needed for the container to function are 
| name | description |
| --- | --- |
| CONFIGNAME | Name of remote to use in Rclone config |
| ADMINKEY | Base key used with root access to the storage remote. Should be a 64 character random string |
| DATABASE_URL | URL of the postgres database to connect to (uses password postgres) |



