#!/bin/bash

set -euo pipefail

bold='\033[1m'
normal='\033[0m'
underline='\033[4m'

validate_kustomization() {
    local kustomization_file="$1"
    local project_root="${2:-$(git rev-parse --show-toplevel)}"
    
    local dir=$(dirname "$kustomization_file")
    local relative_path=${kustomization_file#"$project_root/"}
    local message="${bold}Validating${normal} ${underline}$relative_path${normal}"
    
    echo -n -e "⏳ ${message}"
    if output=$(kustomize build --load-restrictor LoadRestrictionsNone --stack-trace "$dir" 2>&1); then
        echo -e "\r✅ ${message}"
        return 0
    else
        echo -e "\r❌ ${message}"
        echo "$output"
        return 1
    fi
}

validate_all() {
    local project_root="${1:-$(git rev-parse --show-toplevel)}"
    local exit_code=0
    
    while IFS= read -r -d '' kustomization_file; do
        if ! validate_kustomization "$kustomization_file" "$project_root"; then
            exit_code=1
        fi
    done < <(find "$project_root" -name "kustomization.yaml" -type f -print0)
    
    return $exit_code
}

# When script is not sourced, but directly invoked, validate all manifests in the project
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    PROJECT_ROOT=$(git rev-parse --show-toplevel)
    validate_all "$PROJECT_ROOT"
    exit $?
fi
