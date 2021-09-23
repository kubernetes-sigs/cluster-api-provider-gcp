# Copyright 2020 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Provides tools for external cloud provider

CAPO_SCRIPT=create_cloud_conf.sh
while test $# -gt 0; do
	case "$1" in
		-h|--help)
			echo "${CAPO_SCRIPT} - create cloud.conf for Google cloud provider"
			echo " "
		        echo "source ${CAPO_SCRIPT} [options] <path/to/clouds.yaml> <cloud>"
			echo ""
			echo "options:"
		        echo "-h, --help                show brief help"
	                exit 0
          		;;
		*)
			break
			;;
		esac
done

# Check if clouds.yaml file provided
if [[ -n "${1-}" ]] && [[ $1 != -* ]] && [[ $1 != --* ]];then
	CAPO_CLOUDS_PATH="$1"
else
	echo "Error: No clouds.yaml provided"
	echo "You must provide a valid clouds.yaml script to generate a cloud.conf"
	echo ""
	return 1
fi

# Check if os cloud is provided
if [[ -n "${2-}" ]] && [[ $2 != -* ]] && [[ $2 != --* ]]; then
	export CAPO_CLOUD=$2
else
	echo "Error: No cloud specified"
	echo "You must specify which cloud you want to use."
	echo ""
	return 1
fi

CAPO_YQ_TYPE=$(file "$(which yq)")
if [[ ${CAPO_YQ_TYPE} == *"Python script"* ]]; then
	echo "Wrong version of 'yq' installed, please install the one from https://github.com/mikefarah/yq"
	echo ""
	return 1
fi

CAPO_CLOUDS_PATH=${CAPO_CLOUDS_PATH:-""}
CAPO_GCP_CLOUD_YAML_CONTENT=$(cat "${CAPO_CLOUDS_PATH}")

# Just blindly parse the cloud.yaml here, overwriting old vars.
CAPO_AUTH_URL=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.auth.auth_url)
CAPO_USERNAME=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.auth.username)
CAPO_PASSWORD=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.auth.password)
CAPO_REGION=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.region_name)
CAPO_PROJECT_ID=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.auth.project_id)
CAPO_PROJECT_NAME=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.auth.project_name)
CAPO_DOMAIN_NAME=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.auth.user_domain_name)
if [[ "$CAPO_DOMAIN_NAME" = "null" ]]; then
   CAPO_DOMAIN_NAME=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.auth.domain_name)
fi
CAPO_DOMAIN_ID=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.auth.user_domain_id)
if [[ "$CAPO_DOMAIN_ID" = "null" ]]; then
   CAPO_DOMAIN_ID=$(echo "$CAPO_GCP_CLOUD_YAML_CONTENT" | yq r - clouds.${CAPO_CLOUD}.auth.domain_id)
fi

# Build cloud.conf
# Basic cloud.conf, no LB configuration as that data is not known yet.
CAPO_CLOUD_PROVIDER_CONF_TMP=$(mktemp /tmp/cloud.confXXX)
cat >> ${CAPO_CLOUD_PROVIDER_CONF_TMP} << EOF
[Global]
auth-url=${CAPO_AUTH_URL}
username="${CAPO_USERNAME}"
password="${CAPO_PASSWORD}"
EOF

if [[ "$CAPO_PROJECT_ID" != "" && "$CAPO_PROJECT_ID" != "null" ]]; then
   echo "tenant-id=\"${CAPO_PROJECT_ID}\"" >> ${CAPO_CLOUD_PROVIDER_CONF_TMP}
fi
if [[ "$CAPO_PROJECT_NAME" != "" && "$CAPO_PROJECT_NAME" != "null" ]]; then
   echo "tenant-name=\"${CAPO_PROJECT_NAME}\"" >> ${CAPO_CLOUD_PROVIDER_CONF_TMP}
fi
if [[ "$CAPO_DOMAIN_NAME" != "" && "$CAPO_DOMAIN_NAME" != "null" ]]; then
   echo "domain-name=\"${CAPO_DOMAIN_NAME}\"" >> ${CAPO_CLOUD_PROVIDER_CONF_TMP}
fi
if [[ "$CAPO_DOMAIN_ID" != "" && "$CAPO_DOMAIN_ID" != "null" ]]; then
   echo "domain-id=\"${CAPO_DOMAIN_ID}\"" >> ${CAPO_CLOUD_PROVIDER_CONF_TMP}
fi

if [[ "$CAPO_CACERT_ORIGINAL" != "" && "$CAPO_CACERT_ORIGINAL" != "null" ]]; then
   echo "ca-file=\"${CAPO_CACERT_ORIGINAL}\"" >> ${CAPO_CLOUD_PROVIDER_CONF_TMP}
fi
if [[ "$CAPO_REGION" != "" && "$CAPO_REGION" != "null" ]]; then
   echo "region=\"${CAPO_REGION}\"" >> ${CAPO_CLOUD_PROVIDER_CONF_TMP}
fi

cat ${CAPO_CLOUD_PROVIDER_CONF_TMP}
