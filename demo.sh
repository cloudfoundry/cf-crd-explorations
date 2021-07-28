#!/usr/bin/env bash

# echo -ne "${RED}CF Shim for Kubernetes Spike Demo!\n${NC}\n"

echo "
██████ ███████     ███████ ██   ██ ██ ███    ███     ███████  ██████  ██████      ██   ██ ██    ██ ██████  ███████ ██████  ███    ██ ███████ ████████ ███████ ███████ 
██      ██          ██      ██   ██ ██ ████  ████     ██      ██    ██ ██   ██     ██  ██  ██    ██ ██   ██ ██      ██   ██ ████   ██ ██         ██    ██      ██      
██      █████       ███████ ███████ ██ ██ ████ ██     █████   ██    ██ ██████      █████   ██    ██ ██████  █████   ██████  ██ ██  ██ █████      ██    █████   ███████ 
██      ██               ██ ██   ██ ██ ██  ██  ██     ██      ██    ██ ██   ██     ██  ██  ██    ██ ██   ██ ██      ██   ██ ██  ██ ██ ██         ██    ██           ██ 
 ██████ ██          ███████ ██   ██ ██ ██      ██     ██       ██████  ██   ██     ██   ██  ██████  ██████  ███████ ██   ██ ██   ████ ███████    ██    ███████ ███████
"


RED='\033[0;31m'
LIGHTCYAN='\033[1;36m'
LIGHTGREEN='\033[1;32m'
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
    printf "${LIGHTCYAN}Create App${NC}\n"
    echo -ne "${LIGHTGREEN}POST $CF_SHIM_HOST/v3/apps${NC}\n"
    curl -s -v "$CF_SHIM_HOST/v3/apps" -X POST -H "Content-type: application/json" -d "$(generate_app_payload)" | tee $tmp_dir/app.json
    
    printf "${LIGHTCYAN}Response payload${NC}\n"
    cat $tmp_dir/app.json | jq .

    app_guid=$(cat $tmp_dir/app.json | jq -r .guid)
}

function inspect_app {
    printf "${LIGHTCYAN}Inspecting App $app_guid${NC}\n"
    echo -ne "${LIGHTGREEN}GET $CF_SHIM_HOST/v3/apps/$app_guid${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/apps/$app_guid" | jq .
}


function generate_empty_package_payload {
    cat <<EOF
{"type":"bits","relationships":{"app":{"data":{"guid":"$app_guid"}}}}
EOF
}

function create_empty_package {
    printf "${LIGHTCYAN}Create empty Package${NC}\n"
    echo -ne "${LIGHTGREEN}POST $CF_SHIM_HOST/v3/packages${NC}\n"
    curl -s -v "$CF_SHIM_HOST/v3/packages" -X POST -H "Content-type: application/json" -d "$(generate_empty_package_payload)" | tee $tmp_dir/package.json

    printf "${LIGHTCYAN}Response payload${NC}\n"
    cat $tmp_dir/package.json | jq .

    package_guid=$(cat $tmp_dir/package.json | jq -r .guid)
}

function upload_bits_to_package {
    printf "${LIGHTCYAN}Upload bits to Package $package_guid${NC}\n"
    echo -ne "${LIGHTGREEN}POST $CF_SHIM_HOST/v3/packages/$package_guid/upload${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/packages/$package_guid/upload" -X POST -F bits=@"node.zip" | tee $tmp_dir/package.json

    printf "${LIGHTCYAN}Response payload${NC}\n"
    cat $tmp_dir/package.json | jq .
}

function inspect_package {
    printf "${LIGHTCYAN}Inspecting Package $package_guid${NC}\n"
    echo -ne "${LIGHTGREEN}GET $CF_SHIM_HOST/v3/packages/$package_guid${NC}\n"
    printf "${LIGHTCYAN}Response payload${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/packages/$package_guid" | jq .
}

function generate_build_payload {
    cat <<EOF
{"package":{"guid":"$package_guid"}}
EOF
}

function create_build {
    printf "${LIGHTCYAN}Create Build with Package $package_guid${NC}\n"
    echo -ne "${LIGHTGREEN}POST $CF_SHIM_HOST/v3/builds${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/builds" -X POST -H "Content-type: application/json" -d "$(generate_build_payload)" | tee $tmp_dir/build.json

    printf "${LIGHTCYAN}Response payload${NC}\n"
    cat $tmp_dir/build.json | jq .

    build_guid=$(cat $tmp_dir/build.json | jq -r .guid)
}

function inspect_build {
    echo -ne "${LIGHTCYAN}Waiting for Droplet guid${NC}\n"
    echo -ne "${LIGHTGREEN}GET $CF_SHIM_HOST/v3/builds/$build_guid${NC}\n"
    while : ; do
        droplet=$(curl -s -s "$CF_SHIM_HOST/v3/builds/$build_guid" | jq .droplet)
        
        if [ "null" == "$droplet" ]; then
            echo -ne "${RED}."
            sleep 1
        else
            echo -ne "${LIGHTCYAN}Droplet set!${NC}\n"
            break
        fi
    done

    printf "${LIGHTCYAN}Inspecting Build $build_guid${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/builds/$build_guid" | tee $tmp_dir/build_with_droplet.json

    printf "${LIGHTCYAN}Response payload${NC}\n"
    cat $tmp_dir/build_with_droplet.json | jq .
    
    droplet_guid=$(cat $tmp_dir/build_with_droplet.json | jq -r .droplet.guid)
    printf "${LIGHTCYAN}Build produced with Droplet: $droplet_guid${NC}\n"
}

function generate_set_droplet_payload {
    cat <<EOF
{"data":{"guid":"$droplet_guid"}}
EOF
}

function set_droplet {
    printf "${LIGHTCYAN}Setting Droplet $droplet_guid on App $app_guid${NC}\n"
    echo -ne "${LIGHTGREEN}PATCH $CF_SHIM_HOST/v3/apps/$app_guid/relationships/current_droplet${NC}\n"

    curl -s "$CF_SHIM_HOST/v3/apps/$app_guid/relationships/current_droplet" -X PATCH -H "Content-type: application/json" -d "$(generate_set_droplet_payload)" | jq .
}

function start_app {
    printf "${LIGHTCYAN}Starting App $app_guid${NC}\n"
    printf "${LIGHTGREEN}POST $CF_SHIM_HOST/v3/apps/$app_guid/actions/start${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/apps/$app_guid/actions/start" -X POST | jq .
}

function stop_app {
    printf "${LIGHTCYAN}Stopping App $app_guid${NC}\n"
    printf "${LIGHTGREEN}POST $CF_SHIM_HOST/v3/apps/$app_guid/actions/stop${NC}\n"
    curl -s "$CF_SHIM_HOST/v3/apps/$app_guid/actions/stop" -X POST | jq .
}

echo -ne "\n\n\n"
read -n 1 -p "Press any key to create an App"
echo -ne "\n\n\n"
create_app

echo -ne "\n\n\n"
read -n 1 -p "Press any key to create a Package for App $app_guid"
echo -ne "\n\n\n"
create_empty_package

echo -ne "\n\n\n"
read -n 1 -p "Press any key to upload bits to Package $package_guid"
echo -ne "\n\n\n"
upload_bits_to_package

echo -ne "\n\n\n"
read -n 1 -p "Press any key to inspect Package $package_guid"
echo -ne "\n\n\n"
inspect_package

echo -ne "\n\n\n"
read -n 1 -p "Press any key to create Build for Package $package_guid"
echo -ne "\n\n\n"
create_build

echo -ne "\n\n\n"
read -n 1 -p "Press any key to inspect Build $build_guid"
echo -ne "\n\n\n"
inspect_build

echo -ne "\n\n\n"
read -n 1 -p "Press any key to set Droplet $droplet_guid on App $app_guid"
echo -ne "\n\n\n"
set_droplet

echo -ne "\n\n\n"
read -n 1 -p "Press any key to start App $app_guid"
echo -ne "\n\n\n"
start_app

echo -ne "\n\n\n"
read -n 1 -p "Press any key to inspect App $app_guid"
echo -ne "\n\n\n"
inspect_app

echo -ne "\n\n\n"
read -n 1 -p "Press any key to stop App $app_guid"
echo -ne "\n\n\n"
stop_app