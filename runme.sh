#!/bin/bash -e
# Syntax runme.sh [-d|--dryrun] [-h|--help]

DRY_RUN=0
STEP_COUNTER=1
NEW_APP_NAME=
DEFAULT_APP_NAME=
RANDOM_BRANCH=

help() {
    echo """
    > runme.sh [-d|--dryrun] [-h|--help]
    """
}

user_confirmation() {
  read -p "$1" USER_CONFIRMATION

  if [ -z $USER_CONFIRMATION ] || [ $USER_CONFIRMATION = "n" ] || [ $USER_CONFIRMATION = "no" ]
  then
    echo "Aborting..."
    exit 1
  elif [ $USER_CONFIRMATION = "y" ] || [ $USER_CONFIRMATION = "yes" ]
  then
      echo "Proceeding..."
      return 0
  else
      echo "Aborting..."
      exit 2
  fi
}

check_command_exists() {
  CMD=$1
  BREW_PKG=$2
  VERSION_CMD=$3

  if ! $CMD $VERSION_CMD &> /dev/null
  then
    echo "!!! $CMD command not found"
    user_confirmation "Would you like to install it (y/N): "
    brew install $BREW_PKG
  else
      echo "✅ $CMD already installed"
  fi
}

verify_prerequisites() {
  OS_FLAVOR=`uname`
  if [ $OS_FLAVOR = "Darwin" ]
  then
    check_command_exists gsed gnu-sed --version
    check_command_exists xxd xxd --version
  fi
}

emptycheck() {
  if [ -z "$2" ]
  then
    echo
    echo "!!! Error: $1 cannot be empty."
    return "-1"
  else
    return 0
  fi
}

step() {
  echo
  echo "⭐ Step $STEP_COUNTER: $1"
  echo
  STEP_COUNTER=$((STEP_COUNTER+1))
  if [ "$1" == "Done" ]
  then
    echo "My job here is done. Removing myself...good-bye!"
    git rm -f runme.sh
    git commit -q -m "$NEW_APP_NAME: remove runme.sh"
    git push -q origin $RANDOM_BRANCH
  fi
}

read_user_input() {
  return $
}

# detect origin git url and appname
GIT_URL=$(git config --get remote.origin.url)
if [[ $GIT_URL =~ ^git.* ]]
then
tmp=${GIT_URL#"git@"}
tmp=${tmp%".git"}
tmp=${tmp/:/\/}
GIT_URL="https://${tmp}"
fi
DEFAULT_APP_NAME=${GIT_URL##*\/}

get_user_input() {
  step "User Input"
  read -p "☞ Enter new micro-service name [$DEFAULT_APP_NAME]: " NEW_APP_NAME
  NEW_APP_NAME=${NEW_APP_NAME:-$DEFAULT_APP_NAME}
  emptycheck "New micro-service Name" $NEW_APP_NAME
}

while [[ $# -gt 0 ]]
do
  key="${1}"

  case ${key} in
  -d|--dryrun)
    DRY_RUN=1
    shift
    ;;
  -h|--help)
    help
    exit 0
    ;;
  -s|--skip-git)
    SKIP_GIT=1
    shift
    ;;
  *) # unknown
    echo Unknown Parameter $1
    exit 4
  esac
done

echo "*************************************************"
echo "*    Create a ETI SRE template micro-service    *"
echo "*************************************************"
echo

if [ "$DRY_RUN" = 1 ]
then
  echo "!!! DRY RUN ONLY"
fi

if [ "$SKIP_GIT" = 1 ]
then
  echo "!!! SKIP GIT OPS"
fi

step "Verifying Pre-requisites"
verify_prerequisites

get_user_input

step "User Confirmation"
echo "***********************************************************"
echo "New micro-service Name     : $NEW_APP_NAME"
echo "***********************************************************"
echo

echo
user_confirmation "Please confirm to proceed with configuring this directory for \"$NEW_APP_NAME\" (y/N): "

if [ "$DRY_RUN" = 1 ]
then
  echo
  echo "DRY RUN Complete"
  exit 0
fi

if [ -z $SKIP_GIT ]
then
RANDOM_BRANCH_POSTFIX=$(xxd -l 4 -c 4 -p < /dev/random)
RANDOM_BRANCH=$NEW_APP_NAME-$RANDOM_BRANCH_POSTFIX
step "Create a new git branch: $RANDOM_BRANCH"
git checkout -b $RANDOM_BRANCH
fi

step "Update template for $NEW_APP_NAME"
find . -type f ! -name 'runme.sh' ! -name 'README.md' ! -path '*/.git/*' -exec gsed -i "s/sre-go-helloworld/${NEW_APP_NAME}/g" {} +
find . -type f ! -name 'runme.sh' ! -name 'README.md' ! -path '*/.git/*' -exec gsed -i "s/helloworld/${NEW_APP_NAME}/g" {} +
find . -type d -iname '*sre-go-helloworld*' ! -path '*/.git/*' -depth -exec bash -c 'mv "$1" "${1/sre-go-helloworld/'${NEW_APP_NAME}'}"' -- '{}' ';'
find . -type f -iname '*sre-go-helloworld*' ! -path '*/.git/*' -depth -exec bash -c 'mv "$1" "${1/sre-go-helloworld/'${NEW_APP_NAME}'}"' -- '{}' ';'

step "Run gofmt"
gofmt -s -w .

if [ -z $SKIP_GIT ]
then
step "Commit changes to git"
git add .
git commit -m "$NEW_APP_NAME: Executed runme.sh to update template for $NEW_APP_NAME"

step "Push changes to origin branch $RANDOM_BRANCH"
git push origin $RANDOM_BRANCH
fi

step "Done"