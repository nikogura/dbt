# DBT  *Dynamic Binary Toolkit* 

A cross- platform framework for downloading and verifying binary tools.

[![Current Release](https://img.shields.io/github/release/nikogura/dbt.svg)](https://img.shields.io/github/release/nikogura/dbt.svg)

[![Circle CI](https://circleci.com/gh/nikogura/dbt.svg?style=shield)](https://circleci.com/gh/nikogura/dbt)

[![Go Report Card](https://goreportcard.com/badge/github.com/nikogura/dbt)](https://goreportcard.com/report/github.com/nikogura/dbt)

[![Go Doc](https://img.shields.io/badge/godoc-reference-blue.svg?style=flat-square)](http://godoc.org/github.com/nikogura/dbt/pkg/dbt)

[![Coverage Status](https://codecov.io/gh/nikogura/dbt/branch/master/graph/badge.svg)](https://codecov.io/gh/nikogura/dbt)

[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go)  


What kind of tools you say?  Anything that can be compiled into a stand-alone binary.

Tools are always up to date (unless you specify an older version), as is `dbt` itself.  How?  *magic*. 

DBT is basically just a downloader and verifier for executable files.  That's it.  That's all it does.

Say you have a program binary that people use to do their jobs.  How do you distribute it?  How do people stay up to date?  How do they get bug fixes and new versions?

There are a bazillion ways of solving this problem.  At some level, `dbt` is just one more method.  Why is this better than the others?  User Experience.  

DBT gets out of the way and helps the user do what they should be doing all along, but usually don't have time to do - that is verify the integrity and authorship of the tools they use in their daily life.

You absolutely can maintain some sort of device and server management software like Jamf or Chef or Puppet, and update all your OS packages all the time.  These solutions exist, and they're great - when they work well together.  Often they do not- or they're so heavy handed your small agile shop can't maintain them well enough to stay out of the user's way.

Another thing these tools all have in common is they work off a "push model".  The central administrator pushes out updates, and you use whatever you get.  Again this is great - when it works.  Be honest however, when's the last time you were required to update to the latest something and things stopped working?  This month?  This week?  Today.  Yeah.

DBT in contrast works on a _pull model_.  The default is you get the latest version of whatever tool we're talking about.  You can, however, request a previous version instead.  So long as the old versions are available in your repository, the user can do whatever they need to do.  They're the user.  Tools exist to make user's lives easier/better - else what's the use?

Once you set up your tool repository, `dbt` downloads and verifies the tools, automatically looking for and using the latest version - unless the user goes out of their way to use a previous version.  That's it.  That's the magic: Downloading and verifying in a fashion that gets out of the way of the user and lets them do their job.

# Overview

DBT consists of a binary ```dbt``` a config file, and a cache located at ```~/.dbt```.  The ```dbt``` binary checks a trusted repository for tools, which are themselves signed binaries.

Tools are automatically downloaded, and verified for checksum and signature, then if they pass, they're run.

The DBT binary, when run:

  * Fetches the latest truststore from the Repository, verifies its checksum.
  
  * Checks the repository to see what the latest version of `dbt` is.
  
  * Compares its own checksum against the latest version's checksum.
  
  * Auto-updates itself by downloading the latest version, and if the new version's checksum and signature verifies, it overwrites itself on the filesystem, then calls the new binary with the original arguments, letting the child overwrite the parent in the process table.  
  
  * The new binary verifies itself in the same manner.  Presumably at this point, it is the latest version and we can continue.
  
  * Then `dbt` checks the filesystem for the tool indicated, performing the same cycle.
  
  * Checks the Tool on disk, verify checksum against what's in the repository - replacing it if needed.
  
  * Verifies the signature of the Tool - replaces if needed.
  
  * Executes the Tool with the provided arguments, letting the Tool overwrite DBT in the process table, to continue on it's own merry way.

It's rather mind-bending and recursive, but what you get is an always up to date tool with built in authorship and integrity checks.

Tools can be anything and do anything- the only limit is the imagination and skills of the author.  DBT is writen in, and inspired by Golang, where everything compiles down to a single binary, and you can easily cross-compile for other OSes and Architectures, but you don't have to use golang to use `dbt`.

A Tool can be anything that is contained in a single file, and can have a 'detatched signature'.  Since you can digitally 'sign' any digital file, anything completely self contained in a single file is good to go.  Prefer Python?  Build your stuff with PyInstaller or PyOxidizer and DBT will distribute it to your users.  

You could even have a bash script, signed by a trusted source, in a repo that DBT is configured to trust, and it will work.  Pretty cool huh?

This allows the author to make tools that do things that can be truly 'write once, run anywhere' - for any desired degree of 'anywhere'.  Furthermore, the tools and ```dbt``` itself are self-updating, so every time you use it, you're using the latest version available.  

For backwards compatibility and emergencies, you can also specify the version of a tool, and use any old version in your trusted repository.  The default, however will be to get and use the latest.

If the trusted repo is offline, or unavailable, you can choose to degrade gracefully into 'offline mode' and use tools that are already downloaded to disk.  

As an added bonus, the Tools are programs in and of themselves.  There's no difference between a binary program running by itself and one run via ```dbt```.  All ```dbt``` does is download a program, verify it for integrity and authorship, and then exec it with the arguments you provide.

It's all up to you.  DBT is a framework, and frameworks are all about *enablement*. 

# Diagram (requires [mermaid extension](https://chrome.google.com/webstore/detail/github-%20-mermaid/goiiopgdnkogdbjmncgedmgpoajilohe?hl=en))

```mermaid
sequenceDiagram
    participant DBT
    participant Tool
    participant Repository
    DBT-->>Repository: Get truststore from Repository (public keys of trusted tool authors).
    DBT-->>Repository: What's latest version of dbt, and what's it's sha256 checksum?
    loop DBT Integrity Check
        DBT->>DBT: Calculate my own checksum.
        DBT->>DBT: Compare calculated checksum against downloaded checksum.
        DBT->>DBT: Verify signature of DBT itself.
    end
    Note over DBT,Repository: If validation fails, download the latest version.<br>Validate that, and if it passes, execute it with the original arguments.<br> The original process exits.  The child takes over parent's pid.<br>Lather, rinse, and repeat.
    DBT-->>Repository: Is there a tool called <tool name>?
    DBT-->>Repository: What's the latest version of <tool name>, and what's it's sha256 checksum?
    loop Tool Integrity Check
        DBT->>Tool: Is <tool name> already on disk?
        Note over DBT,Repository: If not, download it, it's checksum, and it's signature.
        DBT->>Tool: Calculate sha256 checksum of Tool.
        DBT->>Tool: Compare calculated checksum against downloaded checksum.
        DBT->>Tool: Verify signature of Tool.
    end
    DBT-->>Tool: Run <tool name> with provided arguments.
    Note over DBT,Repository: DBT exits.  Tool takes DBT's pid in the process table.

```
        
So, from the command line, if you were to run: 

    dbt -V -- catalog list -v
    
You would run `dbt` in verbose mode, and `catalog` with flag `-v` and argument `list`

The `--` tells the shell it's done parsing flags and options.  Anything to the right of it are arguments, the first of which is the name of the tool to run, and anything after that gets passed into the tool as the tool's arguments.  Slightly wonky, but very *very* useful.


# Why?

I had an experience in a role where a great many people did not agree with or accept that part of being a conscientious computer user was keeping their systems and tools up to date.  Expecting people to pay attention to upgrade announcements, or even run things like ```brew upgrade``` regularly was not only too much.  It was viewed as offensive, and a failure on my part as a toolsmith.  *wow huh?*

Necessity is, as they say, the mother of invention though, so I worked out a way to slice that particular Gordian Knot.  What I came up with was a way I could make everyone happy.  Users were happy because they didn't need to concern themselves with updates.  I could also make *myself* happy, because I could make sure the system was secure, reliable, and had some fall back.  What I came up with I now offer to you as 'DBT'.

Whether the particular folks that drove me to this extreme were right or wrong is not really important.  It turns out there is actually a legitimate use case for self-updating tooling beyond simply appeasing user laziness.

Sometimes you want to be a _user_ of a tool, and not it's _author_ or _maintainer_.  Those hardy souls that make wonderfully useful and reliable building blocks that the rest of us can use to construct our own towers of awesomeness are to be glorified and revered- it's true.  Face it though, you don't have _time_ to be that person for every library and tool in your bag of tricks.

Are you on the latest version?  What is the latest version?  Do you need to upgrade?  How?  Will the version you have even work?  How do you know when you need a new version? These are questions that sometimes you'd rather not have to ask.  They're also questions that, odds are, the author of the tool is already tired of answering.  Why not let the machine handle it for you?


DBT is doing exactly what you would do, if you had time, resources, and face it, _interest_ in doing it all by hand in a secure fashion. 

Be honest.  How often do you verify the checksum or signature on something you download and run?  Do you even know how to verify them?  Don't feel bad, many people don't. While it's a good thing to know how to do, the syntax for the tools are generally wonky and esoteric.  It's not the sort of thing you're going to remember how to do unless you do it a lot, and who wants that job?  Blech.

DBT does your due diligence for you, and lets you get on with your day.

Another real-world example:  Imagine this, you've got a system of dynamic VM's and Containers, all leveraging common tooling.  You might even have a serious DAG or web of 'things' dynamically generating other 'things' in a busy and automated fashion.  What is there's a problem, or an upgrade?  With normal utility tools and scripts you have to re-bake your machine images and containers to pick up the changes.  You might say that that's a good thing.  But what if it's not?

With DBT, you have the best of both worlds.  You can force your tools to use an explicit version (```dbt -v 1.2.3  -- <tool>```).  You can also dispense with the '-v' and run the latest.  Voila!  You're automatically picking up the latest version of the tooling from your trusted repository.

# Usage

Generally speaking, you will run your tools with a command of the form:

    dbt [flags] -- <command>  <command args and flags>
    
Take special note of the `--`  That double dash separates the flags for `dbt` itself from those of the command.  It can get confusing if you don't spot the double dash and grok it's meaning.

Without it, any flags you try to run on `<command>` will be consumed by `dbt` itself, and the result will probably not be what you intend.

Of course, if your command has no flags itself, only positional arguments, you can run it straight without the double dash.  

# Security

DBT is as secure as the repository you trust to hold the binaries, and the degree to which you protect the signing keys.  It will ensure, come hell or high water that every bit of the binary downloaded is what it aught to be, and that the signature is one you've decided to trust.  If it can't do that, it'll stop- immediately and scream bloody murder.  

You can make the repo wide open, and give everyone a copy of a non-encrypted key and it'll work.  It's just not recommended.  

I just build the tools.  You choose how to use them.

*"If you aim the gun at your foot and pull the trigger, it's UNIX's job to 
ensure reliable delivery of the bullet to where you aimed the gun (in
this case, Mr. Foot)."* -- Terry Lambert, FreeBSD-Hackers mailing list.

# Repository Support

The dbt `reposerver` tool is written entirely in golang.  All the internal tests work off an instance of the dbt reposerver.  See [Reposerver](#reposerver) for more details on how to run it.

[Artifactory Open Source](https://www.jfrog.com/open-source) can be used as a dbt repo.  It works well without auth, or with basic authentication. Paid Artifactory versions work well too.

You can additionally utilize Amazon S3 as a repo server.  Authentication to S3 is assumed to be already in place and leverages the expected configs in ~/.aws.  Credential managers work transparently through `credential_process` as detailed in the AWS docs.

*N.B.* For S3 usage, only Virtual Host based S3 urls are supported.  Why?  Because AWS is deprecating the path-style access to buckets. Check out [https://aws.amazon.com/blogs/aws/amazon-s3-path-deprecation-plan-the-rest-of-the-story/](https://aws.amazon.com/blogs/aws/amazon-s3-path-deprecation-plan-the-rest-of-the-story/) for more information.


# Included Tools

The whole point of DBT is that you'll create your own tools to do things your way.  DBT is itself just a framework, and does exactly *nothing* without the tools that it's designed to download and run.  By itself, it can't even tell you what tools are available to you.  

DBT is designed to be as open and generic as possible. I, the author, don't know what you're going to do with it, and I will make as few assumptions as I possibly can while still presenting you with a useful tool.  

There are, however, some common tasks that any user of DBT might want at their fingertips. The following is a list of tools that will build automatically with dbt and be available for your pleasure:

If for some reason you don't want to use the included tools, just remove them from your `metadata.json` and they won't publish.

## Catalog

A tool for showing what tools are in your repository.

Command: `dbt catalog list`

Output: 

    Fetching information from the repository...
    Commands:
    
    	Command Name		Latest Version		Description
    
    
    
    	    boilerplate    		3.0.4			A tool boilerplate generating tool for DBT.
    	        catalog    		3.0.4			Tool for showing available DBT tools.
    	     certinator    		1.0.0			Instrument Hashicorp Vault to create CA's and Certificates for Clients and Services
    	      genkeyset    		0.1.2			Generate a JWK KeySet for use with Orionlabs PTT
    	     reposerver    		3.0.4			A tool repository server for DBT.
    
    Further information on any tool can be shown by running 'dbt <command> help'.
    
### Catalog Help

Command: `dbt catalog help` 

Output:

    Tool for showing available DBT tools.
    
    DBT tools are made available in a trusted repository.  This tool show's what's available there.
    
    Usage:
      catalog [command]
    
    Available Commands:
      help        Help about any command
      list        ListCatalog available tools.
    
    Flags:
      -h, --help       help for catalog
      -V, --verbose    Verbose output
      -v, --versions   Show all version information for tools.
    
    Use "catalog [command] --help" for more information about a command.


## Boilerplate

A tool for generating tool boilerplate.  You could do it by hand, but why? 

Command: `dbt boilerplate gen`

Output:

    Enter a name for your new tool: fargle
    Enter a go package for your new tool: github.com/nikogura/fargle
    Enter a short, one line description for your new tool: Do cool stuff
    Enter your name: Nik Ogura
    Enter  your email address: nik.ogura@gmail.com
    Enter the dbt tool repository url for your new tool (where compiled tools will be published): https://niks-cool-dbt-repo.com/tools
    Creating /home/nik/fargle/pkg/fargle
    Creating /home/nik/fargle/templates
    Creating /home/nik/fargle/cmd
    Writing /home/nik/fargle/.gitignore
    Writing /home/nik/fargle/pre-commit-hook.sh
    Writing /home/nik/fargle/templates/description.tmpl
    Writing /home/nik/fargle/metadata.json
    Writing /home/nik/fargle/LICENSE
    Writing /home/nik/fargle/go.mod
    Writing /home/nik/fargle/main.go
    Writing /home/nik/fargle/cmd/root.go
    Writing /home/nik/fargle/pkg/fargle/fargle.go
    Writing /home/nik/fargle/README.md

The code generated by the `boilerplate` tool will compile, and publish via `gomason` provided you have `metadata.json` wired up to a repo you can write to.  After that it's up to you.

### Boilerplate Help

Command: `dbt boilerplate help`

Output: 

    DBT tool for creating DBT tools.
    
    Sure, a DBT tool is just a signed go binary.  You can create 'em any old way.
    
    Sometimes, however, you have better things to do with your time, and just want to get something working hence DBT boilerplate.
    
    Usage:
      boilerplate [command]
    
    Available Commands:
      gen         Creates a new DBT tool.
      help        Help about any command
    
    Flags:
      -h, --help   help for boilerplate
    
    Use "boilerplate [command] --help" for more information about a command.

## Reposerver

An http repository server.  It serves up the various dbt tools and components from a file location on disk. 

At present, the repo server supports basic auth via _htpasswd_ file.  Other auth options will be available as time allows.

### Reposerver Config

A JSON file of the form:

    {
	    "address": "my-hostname.com",
        "port": 443,
        "serverRoot": "/path/to/where/you/store/tools",
        "authType": "basic-htpasswd",
        "authGets": false,
        "authOpts": {
            "idpFile": "/path/to/htpasswd/file"
        }
    }

### Running the Reposerver

Command: `dbt reposerver -f /path/to/config`

Output:

    {"level":"info","msg":"Running dbt artifact server on my-hostname.com port 443.  Serving tree at: /var/dbt","time":"2020-11-11T11:18:43-08:00"} 

# Installation
The easiest way to install `dbt` is via a tool called `gomason`. You can build via `go build` and move the files any which way you like, but `gomason` makes it easy.

If you don't want to make any changes to the code or tools:

1. Fork the repo.

1. Change the `metadata.json` file to reflect your own repository setup and preferences.  Specifically you need to change the `repository` and `tool-repository` lines. Commit and push the changes back to your repo. 

1. Install `gomason` via `go get github.com/nikogura/gomason`. Then run `gomason publish`.  If you have it all set up correctly, it should build and install the binary as well as the installer script for your version of DBT together with the tools `catalog`, `boilerplate`, and `reposerver`.

1. Run the installer you built. It'll be found in `<repo>/install_dbt.sh`.  With an HTTP reposerver like Artifactory or DBT's internal server, you can install this script via `curl https://your.repo.host/path/to/install_dbt.sh | bash`.  

    If you're using S3 as your backend, you will have to do it in 2 steps: 1. ` aws s3 cp s3://<your bucket>/install_dbt.sh install_dbt.sh`.  2. `bash install_dbt.sh`.  This 2 step is forced by the aws cli not being able to feed a downloaded object directly to bash.  (Or at least, I haven't figured out how to make it do so - yet!)

1. Verify installation by running: `dbt catalog list`.

## Customization of Boilerplate Templates

The above is fine if you want `dbt` straight out of the box.  If you want to modify the `boilerplate` templates, you'll need to do some extra work.

The variables that are overridable are:

* GITIGNORE_TEMPLATE produces `.gitignore`.

* METATADATA_TEMPLATE produces `metadata.json`.

* PREHOOOK_TEMPLATE produces `pre-commit-hook.sh`.

* GOMODULE_TEMPLATE produces `go.mod`.

* MAINGO_TEMPLATE produces `main.go`.

* ROOTGO_TEMPLATE produces `cmd/root.go`.

* LICENSE_TEMPLATE produces `LICENSE`.

* EMPTYGO_TEMPLATE produces `pkg/<tool name>/<tool name>.go`.

* DESCRIPTION_TEMPLATE produces `templates/description.tmpl`.

* README_TEMPLATE produces `README.md`.
    

To customise `boilerplate` templates:

1. Fork the repo.

1. Change the `metadata.json` file to reflect your own repository setup and preferences.  You need to change the `repository`, `tool-repository`, and `package` lines.

1. Install `gomason` via `go get github.com/nikogura/gomason`.

1. Create new versions of the boilerplate files.

1. Base64 encode your boilerplate files, and export them into ENV vars.  This is crude, but it's the only way I have found to date where you can inject YOUR code as variables into MY code.

1. Add your template overrides by adding 'ldflags' lines to each build target in `metadata.json`.  eg:

    `"ldflags": "-X github.com/nikogura/dbt/pkg/dbt.METADATA_TEMPLATE=${METADATA_TEMPLATE}"`

    Don't forget to commit and push.  Remember `gomason` will pull from the remote repo before building.  It never sees what's in your local clone.  It pulls a fresh clone every time it runs.  By design it's a 'clean room' for your code.

1. Publish via `gomason publish -s`.  A couple of examples that work well:

    1. From a shell run:
    
        `METADATA_TEMPLATE=$(cat metadata.tmpl | base64) gomason publish -vs`

    1. From a Makefile:
    
        ```
        .EXPORT_ALL_VARIABLES:
    
        METADATA_TEMPLATE = $(shell cat metadata.tmpl | base64)
    
        publish:
            @gomason publish -vs
        ```

    You have to skip the tests, since _my_ tests run against _my_ package, which is `github.com/nikogura/dbt`.  They won't run against your forked package unless you laborously go through my code and remove all references to my package.  (That's certainly possible too, though annoying.)

1. Run the installer you built. It'll be found in `<repo>/install_dbt.sh`.  

    With an HTTP reposerver like Artifactory or DBT's internal server, you can install this script via `curl https://your.repo.host/path/to/install_dbt.sh | bash`.  
  
    If you're using S3 as your backend, you will have to do it in 2 steps: 

    1. `aws s3 cp s3://<your bucket>/install_dbt.sh install_dbt.sh`.  
    
    2. `bash install_dbt.sh`.  

    This 2 step is forced by the aws cli not being able to feed a downloaded object directly to bash.  (Or at least, I haven't figured out how to make it do so - yet!)

1. Verify installation by running: `dbt catalog list` .

The details of what all is supported in `metadata.json` can be found in [https://github.com/nikogura/gomason](https://github.com/nikogura/gomason).  

If you run into trouble, run `gomason publish -v` to see what went wrong.  It's wordy, but fairly precise about what it's trying to do.  Typically errors stem from either bad perms in your repository, or typos in `metadata.json`.

If your `metadata.json` has the following:

    "repository": "http://localhost:8081/artifactory/dbt"
    
Then you should see a file `http://localhost:8081/artifactory/dbt/install_dbt.sh`, which you can run with:

        bash -c "$(curl http://localhost:8081/artifactory/dbt/install_dbt.sh)" 
        
And voila!  Your DBT is now installed.

You will, however need to populate the `truststore` file, which by default, with the above config would be located at `http://localhost:8081/artifactory/dbt/truststore`.  This file contains the PEM encoded public keys of the entities you trust to create DBT binaries.  You can edit this file by hand, it's just a bunch of PEM data squashed together.

_AUTHOR'S NOTE: When I personally maintain an internal fork, I set up a clone of the fork with 2 upstreams: 'origin' is my internal fork, and 'upstream' which is the public github.com/nikogura/dbt.  Then I make all my internal changes as required, and when upstream changes, do a `git pull upstream ...`.  Usually the only changes/conflicts are in the `metadata.json`. To maintain private boilerplate templates I put something like: `export VARIABLE_NAME=$(cat file | base64)` in a Makefile or my CI script and let the machine do the tedious parts.  It's crude, but I can only work with what golang gives me.  The goal is to allow any user to run `dbt boilerplate gen` locally and get my organization's templates._

# Configuration

Dbt uses a config file typically located in ~/.dbt/conf/dbt.json

An example dbt config file:

        {
          "dbt": {
            "repository": "http://localhost:8081/dbt",
            "truststore": "http://localhost:8081/dbt/truststore"
          },
          "tools": {
            "repository": "http://localhost:8081/dbt-tools"
          }
          "username": "",
          "password": "",
          "usernamefunc": "echo $USERNAME",
          "passwordfunc": "echo $PASSWORD"
        }
        
It contains sections for the ```dbt``` tool itself, as well as for the tools dbt will download and run.

The individual sections are detailed below.

## dbt

This section applies to the ```dbt``` binary itself.  The ```dbt``` binary doesn't do much in and of itself beyond download , verify, and run tools, but this is where you set the degree of paranoia on the system by setting the truststore.

It's also conceivable that ```dbt``` itself might need a higher level of paranoia than the tools.  It's all up to you.

### repository

Url of the trusted repository.

### truststore

Url of the truststore.  This file contains the public keys of the trusted authors of dbt binaries.  This can be a single ascii armored public key such as:

        -----BEGIN PGP PUBLIC KEY BLOCK-----
        
        mQENBFowLigBCAC++pVrVRRM86Wo8V7XJsOmU2xtBBY5a8ktB1tdpEhzlPWQHObx
        LINj79HE3lRlIFQmxnKcX3I15bzT3yo3XWLyVUsCDA1Mg9JoU2zJ+u3XftdNBg8J
        eRlTiEwZYflxEYZFSyh3TZI2VZxxlINp/jOGG0dpAdKF3sfKxdTRb30lgDr+wIzv
        oncrjX023UQDHoRZ3f+zPpnkubjhwH8jUHLiGsyKvu0XDB0c4y/6yG6vLUMQDuKX
        bkzBtssdLLA6MTur9Q26dQV/DvuNZdHx17vwXSvf/JMKdWcX80fsAJD644KW9DOg
        pgLqtBa4Tfutt3S8ueIHDnPZBKFL0u+Q61xvABEBAAG0HERCVCBUZXN0IFN1aXRl
        IDxkYnRAZGJ0LmNvbT6JAVQEEwEIAD4WIQTdbDzq2B9JD2WAtKLOaEY1/aXTHwUC
        WjAuKAIbAwUJA8JnAAULCQgHAgYVCAkKCwIEFgIDAQIeAQIXgAAKCRDOaEY1/aXT
        H52LCACYqQnVmJRarckqh1//FUFFpXlTcwWV2zGr3CEFRs0BrWEQD7giehFpKoTL
        JOJJSFd4xcbo/9wMXpJ16soK83o48laxkj+2LDUfDylnTVpVI6zVvAseqnt5nbrA
        CWes75FeIHtQ6woDy7K3RHUORNZ+K37MaH3Wmp1TzwY/vATQyWc9qUebGitxWuVD
        RdtTEcq6WniDWAJ5FqhHZ3TV/hK7QPTi1gaHG+yJZeXuajsNo6CLrfJy6H6itEfi
        XKOns2fiGE/pPxjJpfdTOQipFmw68FuNo8i/A0Nc//d43ejcrqAb9fAKOOTZrpw+
        MoqMsFm6V8j+ZN+oKHKSPaD4i6iNuQENBFowLigBCADKSSCJNCY0vPVz8RaCy/uJ
        byiZ4dkEUIFkE4TKFCulG8QUMdfczUtYfuUH4ir5vNsG2vxHqDo7W0CBZ1nZjVW9
        uUy0TrNrVEsPDcMEqn827oK/pqQmlPq6wxGr6qfrMeAnQKKyQpYA0bwWDxwJ6BBb
        0Lw/YyulbLyoCEUPm4Usn+WA8xvUxoWYj/pjg773OLyoznETQiabieNpTmkgad6x
        0mH1mbjT0r0RCR0ZUqL1tjGUAfIEr58AVKvP4vZT8jw4quma2QFKLrSswF/bCXqr
        K/Eqm+S2lDcOUlY35/fZrBt9Mmr8dF00KYWeND0NE0HFB1cpK5bhHKqMSuwOlrbn
        ABEBAAGJATwEGAEIACYWIQTdbDzq2B9JD2WAtKLOaEY1/aXTHwUCWjAuKAIbDAUJ
        A8JnAAAKCRDOaEY1/aXTH63LB/4qt+H+3HNEvaRgigod+srkxyT/nQH1tLSHQtht
        fukuCgNY7J1y/qGroZxZbB6HSJi//64CH0bV0P06nNoDJt2lPJxKA8nuhxiFEZkf
        ACqtJB4W6CUUIZws9YSxVuV84gHZ4g1eQ6mO99R/4jCbhGCebxr0IgPxkulao9Z+
        jjb+fdwRkztLKL5GLpiPnR9TuLPxVTB9rnuXsHlGdT4rUXDUKGVdI+wimjjurwvw
        vAh3MTVCC0qvQq4V1T1yTCYZ+J7p5wrt1UsBCtYKJfKTeAZN9T7Ji3LVr4jUG2Gn
        zHBlhCAdlhsz+4TN+d04QprL2RW86TsIebptwxUscjqJ8lXO
        =b72A
        -----END PGP PUBLIC KEY BLOCK-----
        
The file can consist of multiple public keys such as:

        -----BEGIN PGP PUBLIC KEY BLOCK-----
        
        mQENBFowLigBCAC++pVrVRRM86Wo8V7XJsOmU2xtBBY5a8ktB1tdpEhzlPWQHObx
        LINj79HE3lRlIFQmxnKcX3I15bzT3yo3XWLyVUsCDA1Mg9JoU2zJ+u3XftdNBg8J
        eRlTiEwZYflxEYZFSyh3TZI2VZxxlINp/jOGG0dpAdKF3sfKxdTRb30lgDr+wIzv
        oncrjX023UQDHoRZ3f+zPpnkubjhwH8jUHLiGsyKvu0XDB0c4y/6yG6vLUMQDuKX
        bkzBtssdLLA6MTur9Q26dQV/DvuNZdHx17vwXSvf/JMKdWcX80fsAJD644KW9DOg
        pgLqtBa4Tfutt3S8ueIHDnPZBKFL0u+Q61xvABEBAAG0HERCVCBUZXN0IFN1aXRl
        IDxkYnRAZGJ0LmNvbT6JAVQEEwEIAD4WIQTdbDzq2B9JD2WAtKLOaEY1/aXTHwUC
        WjAuKAIbAwUJA8JnAAULCQgHAgYVCAkKCwIEFgIDAQIeAQIXgAAKCRDOaEY1/aXT
        H52LCACYqQnVmJRarckqh1//FUFFpXlTcwWV2zGr3CEFRs0BrWEQD7giehFpKoTL
        JOJJSFd4xcbo/9wMXpJ16soK83o48laxkj+2LDUfDylnTVpVI6zVvAseqnt5nbrA
        CWes75FeIHtQ6woDy7K3RHUORNZ+K37MaH3Wmp1TzwY/vATQyWc9qUebGitxWuVD
        RdtTEcq6WniDWAJ5FqhHZ3TV/hK7QPTi1gaHG+yJZeXuajsNo6CLrfJy6H6itEfi
        XKOns2fiGE/pPxjJpfdTOQipFmw68FuNo8i/A0Nc//d43ejcrqAb9fAKOOTZrpw+
        MoqMsFm6V8j+ZN+oKHKSPaD4i6iNuQENBFowLigBCADKSSCJNCY0vPVz8RaCy/uJ
        byiZ4dkEUIFkE4TKFCulG8QUMdfczUtYfuUH4ir5vNsG2vxHqDo7W0CBZ1nZjVW9
        uUy0TrNrVEsPDcMEqn827oK/pqQmlPq6wxGr6qfrMeAnQKKyQpYA0bwWDxwJ6BBb
        0Lw/YyulbLyoCEUPm4Usn+WA8xvUxoWYj/pjg773OLyoznETQiabieNpTmkgad6x
        0mH1mbjT0r0RCR0ZUqL1tjGUAfIEr58AVKvP4vZT8jw4quma2QFKLrSswF/bCXqr
        K/Eqm+S2lDcOUlY35/fZrBt9Mmr8dF00KYWeND0NE0HFB1cpK5bhHKqMSuwOlrbn
        ABEBAAGJATwEGAEIACYWIQTdbDzq2B9JD2WAtKLOaEY1/aXTHwUCWjAuKAIbDAUJ
        A8JnAAAKCRDOaEY1/aXTH63LB/4qt+H+3HNEvaRgigod+srkxyT/nQH1tLSHQtht
        fukuCgNY7J1y/qGroZxZbB6HSJi//64CH0bV0P06nNoDJt2lPJxKA8nuhxiFEZkf
        ACqtJB4W6CUUIZws9YSxVuV84gHZ4g1eQ6mO99R/4jCbhGCebxr0IgPxkulao9Z+
        jjb+fdwRkztLKL5GLpiPnR9TuLPxVTB9rnuXsHlGdT4rUXDUKGVdI+wimjjurwvw
        vAh3MTVCC0qvQq4V1T1yTCYZ+J7p5wrt1UsBCtYKJfKTeAZN9T7Ji3LVr4jUG2Gn
        zHBlhCAdlhsz+4TN+d04QprL2RW86TsIebptwxUscjqJ8lXO
        =b72A
        -----END PGP PUBLIC KEY BLOCK-----
        -----BEGIN PGP PUBLIC KEY BLOCK-----
        
        mQENBFpAPGYBCACtRHQZMgHhmETN6X6MCkP7H88jVBSTwhMoZgk0vl6BWK832Uvi
        SMGiZ63uiPkzoUwOtFhexE0QYgKvGPLTm7RWK2aPmsQOk1o+ksFElsRJxT7LzPEM
        g2ci5qAs7q9H8uEntEqfxb9Yn6yiUOLyw6nCrKc9bJN2dCEszpciZoz7AN1ScU+8
        QM3mBw1ToWUB3AMVkd7jJCVloeYprQbqc7pkJBDy9wAISlNRMeLz0PnEuBIrrz8Z
        An1QcqX0PQVWVqNb/duMK5ZszWGK0owfdeSeQiSLK9kvywwo9KZ5qs8XkCUvldZg
        Qv+5mFKK4/+IVReKlnfMvGKRwrGi1oin1mWXABEBAAG0HURCVCBUZXN0IFN1aXRl
        IDxkYnQyQGRidC5jb20+iQFUBBMBCAA+FiEESN604MD5N5baHiTBzGsmLbfIHwUF
        AlpAPGYCGwMFCQPCZwAFCwkIBwIGFQgJCgsCBBYCAwECHgECF4AACgkQzGsmLbfI
        HwX7sQf+Pt6uy3ZZGWZRldGR4qKT/qUCEAc/AG0b2sSlwDt79tEIkprmhMlvNgqF
        DnK5MmAJjZVeR9urWeYeXYk3CH+ZOcR2AOr/15+1LKzkmmVfWGTagQPUFIKkaBdi
        7ymM2wFCvYWG149X1w/ThZ1ZRSCmpuyHiheW/WAE9P1LFTHI+feIjB7Iflmnxkwq
        tPxYxsc07IU0ZIs5+uuerTLlj8gS8acIeGkpNM97m+CiMoL4RLMyt1qFp/lFbbv1
        Td6Dn6vzNL1ZcYGNQ/SHfLTdcnM9GSA5IA1+RyVWb8npFG/sKdPgHMSYOusbdncP
        70qxf+aa/xvZbTvniXtgmhrqog6DJLkBDQRaQDxmAQgA3s4IfL2wUwRN0aCKFeuW
        yOEgNdMuIS9ZKA+f8/s+VXoLtJQ32gZpqCEl2ESOggJx7+ThAhf4F/SDnEdRHZeJ
        IhUnRmzzQPSrhWo9UWDff6cooO1CiSGDSRBgAT8RvZib3OnRWSe67s0webpytUiO
        +y4gt0FtEhXC9kdJ1DsjzJGVyXFYR9pOTV3xjfGKBhHH/6c6kYr1UL2boy8t3IZP
        jyhBHrp3EgxYV7g96ncAVXha91mZ6IisGyXtsOL5qEwPPJCKD2QTKwkJ4S2qqcAR
        8n7agRD8Cn2HESgPezXeg2uoaStcHzhNhF/o/71j2oj5c2u5HkchAzj3l+XHQIrs
        VwARAQABiQE8BBgBCAAmFiEESN604MD5N5baHiTBzGsmLbfIHwUFAlpAPGYCGwwF
        CQPCZwAACgkQzGsmLbfIHwVwjAf9GWR6LdtoEXYVRUSSB2ccl2IeRiwcaEZl/96A
        I2hFX+SCqBVnwJN4jgvhPlCF6PXylkjZKUczrAaizjuU2ZuAt6ONDkEc0R5Glt7j
        dgyl/51WEdBbYeLuVfONtAOBqzs3iRrK8WHnoV+SYQy5aT4kTPTDVzrz/EBDF7KQ
        jqZ4J0i6qsp2DiOxhPn/xVk/iaTRDvtvsA37Qw0mqRlf6xSSLQabroNtJENmf7Cc
        f51a/98jWbflcGLSg/BG2K4hba7ZNKIgKYrS+SKqx5YeE70y/rbjQcJ0ai09Fojc
        hxfIreyexqK3w7pLJFaTbs4ykxWvQZyF0s7h60THq9g76lTjLQ==
        =KIOK
        -----END PGP PUBLIC KEY BLOCK-----
        
There's nothing magical about this file.  It's just the keys you've decided to trust, concatenated together.  Comments after an `-----END PGP PUBLIC KEY BLOCK-----` or before an `-----BEGIN PGP PUBLIC KEY BLOCK---` are ignored, and can be quite useful for humans trying to maintain this file.

## tools

This section is for the tools ```dbt``` downloads, verifies, and runs for you.

### repository

Url of the repo where the tools are stored.  This is where tools are found, and where the tool ```catalog``` looks for tools.

## username

Username if basic auth is used on repos.  (Optional)

## password

Password if basic auth is used on repos. (Optional)

## usernamefunc

Shell function to retrieive username.

## passwordfunc

Shell funciton to retrieve password.

