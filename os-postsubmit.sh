#!/bin/bash

# Ensure repository URL is provided
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 <repository_url> <branch_name>"
    exit 1
fi

repo_url=$1
branch_name=$2
endpoint="https://gangway-ci.apps.ci.l2s4.p1.openshiftapps.com/v1/executions"

# Extract organization and repo name from the URL
org=$(echo "$repo_url" | awk -F'/' '{print $(NF-1)}')
repo=$(echo "$repo_url" | awk -F'/' '{print $NF}' | sed 's/.git$//')

# Get the latest SHA for the branch
base_sha=$(git ls-remote "$repo_url" "$branch_name" | awk '{print $1}')

# Ensure SHA was retrieved successfully
if [ -z "$base_sha" ]; then
    echo "Error: Failed to retrieve base SHA for branch '$branch_name' in repository '$repo_url'"
    exit 1
fi

# Generate JSON output
json_output=$(cat <<EOF
{
  "job_name": "branch-ci-$org-$repo-$branch_name-okd-scos-images",
  "job_execution_type": "2",
  "refs": {
    "base_link": "$repo_url/compare/$branch_name",
    "base_ref": "$branch_name",
    "base_sha": "$base_sha",
    "org": "$org",
    "repo": "$repo",
    "repo_link": "$repo_url"
  }
}
EOF
)

# Send POST request with JSON payload
response=$(curl -s -w "%{http_code}" -o /dev/null -X POST "$endpoint" \
    -H "Authorization: Bearer $(oc whoami -t)" \
    -D response_headers.txt \
    -d "$json_output")

if [ "$response" -ne 200 ]; then
    echo "Error: status code is $response"

    # Check if the "grpc-message" header exists in the response
    grpc_message=$(grep -i "grpc-message" response_headers.txt)

    if [ -n "$grpc_message" ]; then
        echo "grpc-message header: $grpc_message"
    else
        echo "No grpc-message header found."
    fi
    exit 1
fi

# Print successful response
echo "Postsubmit triggered successfully!"
echo "Response: $response"
