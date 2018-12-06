#!/usr/bin/env bash
set -euo pipefail
unset CDPATH; cd "$( dirname "${BASH_SOURCE[0]}" )"; cd "$(pwd -P)"

# Project settings: package name, test packages (if different), Go & Glide versions, and cross-compilation targets
pkg="github.com/kofalt/memoize"
testPkg=""
coverPkg=""
goV=${GO_VERSION:-"1.11.2"}
minGlideV="0.13.2"
targets=( "linux/amd64" "darwin/amd64" "windows/amd64" )
#

fatal() { echo -e "$1"; exit 1; }

# Check that this project is in a gopath
test -d ../../../../src || fatal "This project must be located in a gopath.\\nTry cloning instead to \"src/$pkg\"."
export GOPATH; GOPATH=$(cd ../../../../; pwd); unset GOBIN

# Get system info
localOs=$( uname -s | tr '[:upper:]' '[:lower:]' )

# Load GNU coreutils on OSX
if [[ "$localOs" == "darwin" ]]; then
	# Check requirements: g-prefixed commands are available if brew packages are installed.
	hash brew gsort gsed gfind 2>/dev/null || fatal "On OSX, homebrew is required. Install from https://brew.sh\\nThen, run 'brew install bash coreutils findutils gnu-sed' to install the necessary tools."

	# Load GNU coreutils, findutils, and sed into path.
	# The GNU versions of these commands are always found via this path suffix.
	suffix="libexec/gnubin"

	# As of at least homebrew 0.9.5, the --prefix command accepts multiple packages.
	# This approach allows us to invoke homebrew once instead of N times, saving several seconds.
	# There is a trailing ':', so no need to add one before postpending $PATH.
	export PATH
	PATH=$(brew --prefix coreutils findutils gnu-sed | tr '\n' ':' | sed "s#:#/$suffix:#g")$PATH

	# OSX has shasum. CentOS has sha1sum. Ubuntu has both.
	alias sha1sum="shasum -a 1"
fi

prepareJunitGenerator() {
	MakeGenerateJunit=${MakeGenerateJunit:-}

	if [ -z "$MakeGenerateJunit" ]; then
		return 0
	else
		go get -v -u github.com/jstemmer/go-junit-report
	fi
}

prepareGo() {
	# Configure gimme: get our desired Go version with reasonable logging, only binary downloads, and local state folder
	export GIMME_GO_VERSION=$goV; export GIMME_SILENT_ENV=1; export GIMME_DEBUG=1
	export GIMME_TYPE="binary"; export GIMME_TMP="./.gimme-tmp"

	# Inherit or set the source directory
	: "${GIMME_ENV_PREFIX:=${HOME}/.gimme/envs}"
	src="${GIMME_ENV_PREFIX}/go${goV}.env"

	# Show download & extract progress, removing other commands, empty lines, and rewrite error message
	filterLog='/^\+ (curl|wget|fetch|tar|unzip)/p; /^\++ /d; /^(unset|export) /d; /(using type .*)/d; /^$/d;'
	filterError='s/'"I don't have any idea what to do with"'/Download or install failed for go/g;'

	# Install go, clearing tempdir before & after, with nice messaging.
	test -f "$src" || (
		echo "Downloading go $goV..."
		rm -rf $GIMME_TMP; mkdir -p $GIMME_TMP

		curl -sL https://raw.githubusercontent.com/travis-ci/gimme/master/gimme > $GIMME_TMP/gimme.sh
		chmod +x $GIMME_TMP/gimme.sh

		$GIMME_TMP/gimme.sh 2>&1 | sed -r "$filterLog $filterError"
		rm -rf $GIMME_TMP
	)

	# Load installed go and prepare for compiled tools
	source "$src"
	export PATH=$GOPATH/bin:$PATH

	test -x "$GOPATH/bin/go-junit-report" || prepareJunitGenerator
}

glideClean() {
	# Cut down on glide chatter
	egrep -v '(Lock file may be out of date|Found desired version locally|Setting version for)'
}

cleanGlideLockfile() {
	# Remove timestamp and hash from glide lockfiles. Pollutes diff, and does not prevent normal operation.
	sed -i '/^updated: /d; /^hash: /d' glide.lock
}

prepareGlide() {
	prepareGo

	# Cache glide install runs by hashing state
	glideHashFile=".glidehash"

	installGlide() {
		echo "Downloading glide $minGlideV or higher..."
		mkdir -p "$GOPATH/bin"
		rm -f "$GOPATH/bin/glide"
		curl -sL https://glide.sh/get | bash
	}

	generateGlideHash() {
		cleanGlideLockfile
		cat glide.lock glide.yaml | sha1sum | cut -f 1 -d ' '
	}

	runGlideInstall() {
		# Whenever glide runs, update the hash marker
		glide install
		generateGlideHash > $glideHashFile
	}

	test -x "$GOPATH/bin/glide" || installGlide

	# Check the current glide version against the minimum project version
	currentVersion=$(glide --version | cut -f 3 -d ' ' | tr -d 'v')
	floorVersion=$(echo -e "$minGlideV\\n$currentVersion" | sort -V | head -n 1)

	if [[ "$minGlideV" != "$floorVersion" ]]; then
		echo "Glide $currentVersion is older than required minimum $minGlideV; upgrading..."
		installGlide
	fi

	# If glide components are missing, or cache is out of date, run
	test -f glide.lock -a -d vendor -a -f $glideHashFile || runGlideInstall
	test "$(cat $glideHashFile)" = "$(generateGlideHash)" || runGlideInstall
}

build() {
	package=${1:-$pkg}
	extraLdFlags=${2:-}

	# Check glide state, unless this is called from cross-build, which does so once already.
	if [ -z "$extraLdFlags" ]; then
		prepareGlide
	fi

	# Clean out the absolute-path pollution, and highlight source code filenames.
	filterAbsolutePaths="s#$GOROOT/src/##g; s#$GOPATH/src/##g; s#$pkg/vendor/##g; s#$PWD/##g;"
	highlightGoFiles="s#[a-zA-Z0-9_]*\.go#$(tty -s && tput setaf 1 2>/dev/null || true)&$(tty -s && tput sgr0 2>/dev/null || true)#;"

	# Go install uses $GOPATH/pkg to cache built files. Faster than go build.
	#
	# Adding -ldflags '-s' strips the the DWARF symbol table and debug information.
	# https://golang.org/cmd/link
	#
	# One downside to this when cross-compiling is that it requires a writable $GOROOT.
	# The only alternative is *recompiling the standard library N times every build*.
	# Gimmie, provisioned above, provides a safely writable Go install in your homedir.
	# https://dave.cheney.net/2015/08/22/cross-compilation-with-go-1-5
	go install -v -ldflags "-s $extraLdFlags" "$package" 2>&1 | sed "$filterAbsolutePaths $highlightGoFiles"
}

cross() {
	package=${1:-$pkg}
	prepareGlide

	# Placed here, instead of at the top, since it's the only place we need it
	localArch=$( uname -m | sed 's/x86_//; s/i[3-6]86/32/' )

	# For release builds, detect useful information about the build.
	# Fails safely & silently. Declare & use these strings in your main!
	BuildHash=$( hash git  2> /dev/null && git rev-parse --short HEAD 2>/dev/null || echo "unknown" )
	BuildDate=$( hash date 2> /dev/null && date "+%Y-%m-%d %H:%M"     2>/dev/null || echo "unknown" )
	# Datestamp is ISO 8601-ish, without seconds or timezones.

	# Versions of UPX prior to 3.92 had compatibility issues with OSX 10.12 Sierra.
	# Don't compress the binaries unless UPX is present and new enough.
	#
	# https://upx.github.io/upx-news.txt
	# https://apple.stackexchange.com/questions/251808/this-upx-compressed-binary-contains-an-invalid-mach-o-header-and-cannot-be-load
	minUPXv="3.92"
	useUPX=false

	# Check UPX version
	if hash upx 2>/dev/null; then
		currentUPXv=$(upx --version | head -n 1 | cut -f 2 -d ' ')
		floorVersion=$(echo -e "$minUPXv\\n$currentUPXv" | sort -V | head -n 1)

		if [[ "$minUPXv" == "$floorVersion" ]]; then
			useUPX=true
		else
			echo "Warning: your UPX version is too old and cannot compress OSX binaries correctly. Disabling."
		fi
	fi

	for target in "${targets[@]}"; do

		# Split target on slash to get operating system & architecture
		IFS='/'; targetSplit=($target); unset IFS;
		os=${targetSplit[0]}; arch=${targetSplit[1]}

		echo -e "\n-- Building $os $arch --"
		GOOS=$os GOARCH=$arch build "$package" "-X main.BuildHash=$BuildHash -X 'main.BuildDate=$BuildDate'"


		if $useUPX; then
			if [[ "$localOs" == "$os" && "$arch" =~ .*$localArch ]] ; then
				path="$GOPATH/bin/"
			else
				path="$GOPATH/bin/${os}_${arch}"
			fi

			binary=$( find "$path" -maxdepth 1 | grep -E "${pkg##*/}(\.exe)*" | head -n 1 )
			nice upx -q "$binary" 2>&1 | grep -- "->" || true
		fi

		# If this system is the current build target, copy the binary to a build folder.
		# Makes it easier to export a cross-build.
		if [[ "$localOs" == "$os" && "$arch" =~ .*$localArch ]] ; then
			path="$GOPATH/bin/"
			binary=$( find "$path" -maxdepth 1 | grep -E "${pkg##*/}(\.exe)*" | head -n 1 )

			mkdir -p "$GOPATH/bin/${os}_${arch}/"
			cp "$binary" "$GOPATH/bin/${os}_${arch}/"
		fi
	done

	hash upx 2>/dev/null || ( echo "UPX is not installed; did not compress binaries." )
}

prepareCrossBuild() {
	prepareGo

	flag="$GOROOT/.custom-flags/stdlib-cross-compiled"
	test -f "$flag" && return 0

	# Filter out packages that are esoteric, require linking, unexported, or outside the stdlib
	template='{{.ImportPath}}${{.Standard}}'
	filter='^(cmd|crypto/x509|debug|go/(build|types)|plugin|runtime|syscall|testing)|internal|vendor|^(net|os/user)$|false$'
	packages=( $( go list -f "$template" ... | grep -v -E $filter | cut -f 1 -d '$' ) )

	# Create a dummy Go file that imports every package, so we can cross-compile them ¯\_(ツ)_/¯
	# This is mainly useful if you'd like to generate these object files, and then cache them for later.
	# Results in faster cross-compile for your CI build!
	folder="crossCompileStdlib"; mkdir -p "$folder"
	tempfile="$folder/main.go"
	echo -e "package main\\nimport (" > $tempfile
	for package in "${packages[@]}"; do echo "	_ \"$package\"" >> $tempfile; done
	echo -e ")\\nfunc main() { }" >> $tempfile

	cross "$pkg/crossCompileStdlib"
	rm -rf $folder
	find "$GOPATH/bin" -executable -type f | grep "crossCompileStdlib" | xargs rm
	mkdir -p "$(dirname "$flag")"; touch "$flag"
}

# Some go tools take package names. Some take file names. Some like the pacakge prefix. Some don't.
# Starting with go 1.9, at least the alias "./..." omits the vendor directory:
# https://github.com/golang/go/issues/19090
# The below solutions assume at least 1.9. Before that, this was more work.

listPackages()     { prepareGo; go list ./...;                                         }
listPackageNames() { prepareGo; go list ./... | sed -r 's#^'"$pkg"'(/)?##g; /^\s*$/d'; }
listBaseFiles()    { find . -maxdepth 1 -type f -name '*.go' | sed 's#^\./##g';        }

format() {
	prepareGo
	gofmt -w -s $(listPackageNames && listBaseFiles)
}

formatCheck() {
	prepareGo
	badFiles=($(gofmt -l -s $(listPackageNames && listBaseFiles)))
	test "${#badFiles[@]}" -eq 0 || fatal "The following files need formatting: ${badFiles[*]}"
}

_test() {
	prepareGlide
	filterTestOutput="/\[no test files\]$/d; /^warning\: no packages being tested depend on /d; /^=== RUN   /d;"
	MakeGenerateJunit=${MakeGenerateJunit:-}

	# Some helper functions
	runTests()                 { go test -v -cover "$@"; }
	runTestsWithCoverProfile() { runTests -coverprofile=.coverage.out -coverpkg $coverPkg "$@"; }
	filterTestOutput()         { sed -r "$filterTestOutput"; }
	generateHtml()             { go tool cover -html=.coverage.out -o coverage.html; rm -f .coverage.out; }

	# Ignore junit XML generation unless specifically enabled
	if [ -z "$MakeGenerateJunit" ]; then
		generateJunit() { cat > /dev/null; }
	else
		generateJunit() {
			# go-junit-report is incompatible with go test's -coverPkg flag.
			# https://github.com/jstemmer/go-junit-report/issues/59
			filterPackageName='s$^(coverage: [0-9\.]*% of statements) in .*$\1$;'

			sed -r "$filterPackageName" | go-junit-report -go-version "$goV" -set-exit-code > .report.xml;
		}
	fi

	# If testing a single package, coverprofile is availible.
	# Set which package to test and which package to count coverage against.

	if [[ $testPkg == "" ]]; then
		runTests "$@" $(listPackages) 2>&1 | tee >(generateJunit) | filterTestOutput
	else
		runTestsWithCoverProfile "$@" $testPkg 2>&1 | tee >(generateJunit) | filterTestOutput
		generateHtml
	fi
}

clean() {
	# Remove all build state
	prepareGo
	rm -rf "${GOPATH:?}/pkg"
	rm -rvf "${GOPATH:?}/bin/${pkg##*/}"
}

showEnv() {
	prepareGlide 1>&2
	(go env; echo "PATH=$PATH") | sed 's/^/export /g'
}

cmd=${1:-"build"}; shift || true
case "$cmd" in
	"go" | "godoc" | "gofmt")
		prepareGo; $cmd "$@";;

	"glide")
		prepareGlide; glide "$@" 2> >(glideClean); cleanGlideLockfile;;

	"test")
		_test "$@";;

	"env") # Load environment!   eval $(./make.sh env)
		showEnv;;

	"goserve") # Run godoc in server mode and open the docs
		prepareGo
		echo "Serving documentation on http://localhost:6060 ..."
		( sleep 0.5; hash firefox 2> /dev/null && firefox http://localhost:6060/pkg/$pkg ) &
		godoc -http :6060;;

	*)
		type "${cmd}" >/dev/null 2>&1 && eval "${cmd}" || (
			echo "Usage: ./make.sh {go|godoc|gofmt|glide|build|format|clean|test|env|ci|cross}"
			exit 1
		);;
esac
