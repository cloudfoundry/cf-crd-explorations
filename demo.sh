#!/usr/bin/env bash

HEADING='\033[0;33m'
SUBHEADING='\033[1;36m'
REQUEST_ENDPOINT='\033[1;32m'
PROGRESS='\033[1;34m'

NC='\033[0m'

CF_SHIM_HOST=$1

tmp_dir=$(mktemp -d -t cf-shim-XXXXXXXXXX)

declare app_guid
declare package_guid
declare build_guid
declare droplet_guid

function generate_app_payload {
    cat <<EOF
{"name":"my-app","relationships":{"space":{"data":{"guid":"default"}}}}
EOF
}

function create_app {
    printf "${HEADING}Create App${NC}\n"
    echo -e "${SUBHEADING}Request endpoint${NC}"
    echo -e "${REQUEST_ENDPOINT}POST $CF_SHIM_HOST/v3/apps${NC}"
    echo -e "${SUBHEADING}Request payload${NC}"
    generate_app_payload | jq .
    curl -s "$CF_SHIM_HOST/v3/apps" -X POST -H "Content-type: application/json" -d "$(generate_app_payload)" > $tmp_dir/app.json
    
    printf "${SUBHEADING}Response payload${NC}\n"
    cat $tmp_dir/app.json | jq .

    app_guid=$(cat $tmp_dir/app.json | jq -r .guid)
}

function inspect_app {
    printf "${SUBHEADING}Inspecting App $app_guid${NC}\n"
    echo -e "${REQUEST_ENDPOINT}GET $CF_SHIM_HOST/v3/apps/$app_guid${NC}"
    curl -s "$CF_SHIM_HOST/v3/apps/$app_guid" | jq .
}


function generate_empty_package_payload {
    cat <<EOF
{"type":"bits","relationships":{"app":{"data":{"guid":"$app_guid"}}}}
EOF
}

function create_empty_package {
    printf "${HEADING}Create empty Package${NC}\n"
    echo -e "${REQUEST_ENDPOINT}POST $CF_SHIM_HOST/v3/packages${NC}"
    echo -e "${SUBHEADING}Request payload${NC}"
    generate_empty_package_payload | jq .
    curl -s "$CF_SHIM_HOST/v3/packages" -X POST -H "Content-type: application/json" -d "$(generate_empty_package_payload)" > $tmp_dir/package.json

    printf "${HEADING}Response payload${NC}\n"
    cat $tmp_dir/package.json | jq .

    package_guid=$(cat $tmp_dir/package.json | jq -r .guid)
}

function upload_bits_to_package {
    printf "${HEADING}Upload bits to Package $package_guid${NC}\n"
    echo -e "${REQUEST_ENDPOINT}POST $CF_SHIM_HOST/v3/packages/$package_guid/upload${NC}"
    curl -s "$CF_SHIM_HOST/v3/packages/$package_guid/upload" -X POST -F bits=@"node.zip" > $tmp_dir/package.json

    printf "${SUBHEADING}Response payload${NC}\n"
    cat $tmp_dir/package.json | jq .
}

function inspect_package {
    printf "${HEADING}Inspecting Package $package_guid${NC}\n"
    echo -e "${REQUEST_ENDPOINT}GET $CF_SHIM_HOST/v3/packages/$package_guid${NC}"
    printf "${SUBHEADING}Response payload${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/packages/$package_guid" | jq .
}

function generate_build_payload {
    cat <<EOF
{"package":{"guid":"$package_guid"}}
EOF
}

function create_build {
    printf "${HEADING}Create Build with Package $package_guid${NC}\n"
    echo -e "${REQUEST_ENDPOINT}POST $CF_SHIM_HOST/v3/builds${NC}"
    echo -e "${SUBHEADING}Request payload${NC}"
    generate_build_payload | jq .
    curl -s "$CF_SHIM_HOST/v3/builds" -X POST -H "Content-type: application/json" -d "$(generate_build_payload)" > $tmp_dir/build.json

    printf "${SUBHEADING}Response payload${NC}\n"
    cat $tmp_dir/build.json | jq .

    build_guid=$(cat $tmp_dir/build.json | jq -r .guid)
}

function inspect_build {
    echo -e "${HEADING}Waiting for Droplet guid${NC}"
    echo -e "${REQUEST_ENDPOINT}GET $CF_SHIM_HOST/v3/builds/$build_guid${NC}"
    while : ; do
        droplet=$(curl -s -s "$CF_SHIM_HOST/v3/builds/$build_guid" | jq .droplet)
        
        if [ "null" == "$droplet" ]; then
            echo -ne "."
            sleep 1
        else
            echo -e "${SUBHEADING}Droplet set!${NC}"
            break
        fi
    done

    printf "${HEADING}Inspecting Build $build_guid${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/builds/$build_guid" | tee $tmp_dir/build_with_droplet.json

    printf "${SUBHEADING}Response payload${NC}\n"
    cat $tmp_dir/build_with_droplet.json | jq .
    
    droplet_guid=$(cat $tmp_dir/build_with_droplet.json | jq -r .droplet.guid)
    printf "${SUBHEADING}Build produced with Droplet: $droplet_guid${NC}\n"
}

function generate_set_droplet_payload {
    cat <<EOF
{"data":{"guid":"$droplet_guid"}}
EOF
}

function set_droplet {
    printf "${HEADING}Setting Droplet $droplet_guid on App $app_guid${NC}\n"
    echo -e "${REQUEST_ENDPOINT}PATCH $CF_SHIM_HOST/v3/apps/$app_guid/relationships/current_droplet${NC}"
    echo -e "${SUBHEADING}Request payload${NC}"
    generate_set_droplet_payload | jq .

    curl -s "$CF_SHIM_HOST/v3/apps/$app_guid/relationships/current_droplet" -X PATCH -H "Content-type: application/json" -d "$(generate_set_droplet_payload)" | jq .
}

function start_app {
    printf "${HEADING}Starting App $app_guid${NC}\n"
    printf "${REQUEST_ENDPOINT}POST $CF_SHIM_HOST/v3/apps/$app_guid/actions/start${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/apps/$app_guid/actions/start" -X POST | jq .
}

function stop_app {
    printf "${HEADING}Stopping App $app_guid${NC}\n"
    printf "${REQUEST_ENDPOINT}POST $CF_SHIM_HOST/v3/apps/$app_guid/actions/stop${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/apps/$app_guid/actions/stop" -X POST | jq .
}

echo -e "\n\n"
read -n 1 -p "Press any key to create an App"
echo -e "\n\n"
create_app

echo -e "\n\n"
read -n 1 -p "Press any key to create a Package for App $app_guid"
echo -e "\n\n"
create_empty_package

echo -e "\n\n"
read -n 1 -p "Press any key to upload bits to Package $package_guid"
echo -e "\n\n"
upload_bits_to_package

echo -e "\n\n"
read -n 1 -p "Press any key to inspect Package $package_guid"
echo -e "\n\n"
inspect_package

echo -e "\n\n"
read -n 1 -p "Press any key to create Build for Package $package_guid"
echo -e "\n\n"
create_build

echo -e "\n\n"
read -n 1 -p "Press any key to inspect Build $build_guid"
echo -e "\n\n"
inspect_build

echo -e "\n\n"
read -n 1 -p "Press any key to set Droplet $droplet_guid on App $app_guid"
echo -e "\n\n"
set_droplet

echo -e "\n\n"
read -n 1 -p "Press any key to start App $app_guid"
echo -e "\n\n"
start_app

echo -e "\n\n"
read -n 1 -p "Press any key to inspect App $app_guid"
echo -e "\n\n"
inspect_app

echo -e "\n\n"
read -n 1 -p "Press any key to stop App $app_guid"
echo -e "\n\n"
stop_app