# Steps to follow a happy path workflow with the CF Shim. Guids will be replaced with new tokens as you follow the steps.


1. Create App
`curl "localhost:9000/v3/apps" -X POST -H "Content-type: application/json" -d '{"name":"my-app","relationships":{"space":{"data":{"guid":"default"}}}}'`

1. Create empty Package
`curl "localhost:9000/v3/packages" -X POST -H "Content-type: application/json" -d '{"type":"bits","relationships":{"app":{"data":{"guid":"ecc9f386-0b15-48cc-a297-f3e1fe53c1d4"}}}}'`

1. Upload Package bits with Package guid
`curl "localhost:9000/v3/packages/ec1f6306-f9bd-4e64-9e55-ffcfc29bed9e/upload" -X POST -F bits=@"node.zip"`

1. Get Package
`curl "localhost:9000/v3/packages/ec1f6306-f9bd-4e64-9e55-ffcfc29bed9e"`

1. Create a Build
`curl "localhost:9000/v3/builds" -X POST -H "Content-type: application/json" -d '{"package":{"guid":"ec1f6306-f9bd-4e64-9e55-ffcfc29bed9e"}}'`

1. Get the Build and grab Droplet guid
`curl "localhost:9000/v3/builds/d7600b6f-b3b1-429e-815b-80ae86f88286"`

1. Set current Droplet
`curl "localhost:9000/v3/apps/ecc9f386-0b15-48cc-a297-f3e1fe53c1d4/relationships/current_droplet" -X PATCH -H "Content-type: application/json" -d '{"data":{"guid":"droplet-d7600b6f-b3b1-429e-815b-80ae86f88286"}}'`

1. Show Process in `kubectl`

1. Start App
`curl "localhost:9000/v3/apps/ecc9f386-0b15-48cc-a297-f3e1fe53c1d4/actions/start" -X POST`

1. Stop App
`curl "localhost:9000/v3/apps/ecc9f386-0b15-48cc-a297-f3e1fe53c1d4/actions/stop" -X POST`