# DBT  *Distributed Binary Toolkit*

A framework for running self-updating, signed binary tools from a central, trusted repository.

# Why?

I had a recent experience where a great many people did not agree with or accept that part of being a conscientious computer user was keeping their systems and tools up to date.  Expecting people to pay attention to upgrade announcements, or even run things like ```brew upgrade``` regularly was not only too much.  It was viewed as offensive, and a failure on my part.  *wow huh?*

Necessity is, as they say, the mother of invention though, so I worked out a way to slice that particular Gordian Knot.  What I came up with was a way I could make everyone happy- because they didn't need to concern themselves with updates.  I could also make *myself* happy, because I could make sure the system was secure, reliable, and had some fall back.  What I came up with I now offer to you as 'DBT'.

Whether the particular folks that drove me to this extreme were right or wrong is not really important.  It turns out there is actually a legitimate use case for self-updating tooling beyond appeasing user laziness.

Imagine this, you've got a system of dynamic VM's and Containers, all leveraging common tooling.  You might even have a serious DAG or web of 'things' dynamically generating other 'things' in a busy and automated fashion.  What is there's a problem, or an upgrade?  With normal utility tools and scripts you have to re-bake your machine images and containers to pick up the changes.  You might say that that's a good thing.  But what if it's not?

With DBT, you have the best of both worlds.  You can force your tools to use an explicit version (```dbt -v 1.2.3 <tool>```).  You can also dispense with the '-v' and run the latest.  Voila!  You're automatically picking up the latest version of the tooling from your trusted repository.

# Overview

DBT consists of a binary ```dbt``` and a config file.  The ```dbt``` binary checks a trusted repository for tools, which are themselves signed binaries.

Tools are automatically downloaded, and verified for checksum and signature before running.

The DBT binary itself auto-updates from the trusted repository, and if it's checksum and signature checks out, it executes a 'Single White Female' on itself, replacing itself on the fly with the new version and then running the downloaded tool.

Tools can be anything and do anything- the only limit is the imagination and skills of the author.  Due to the nature of the Go language, everything compiles down to a single binary, and you can easily cross-compile for other OSes and Architectures.

This allows the author to make tools that do things that can be truly 'write once, run anywhere' - for any desired degree of 'anywhere'.  Furthermore, the tools and dbt itself are self-updating, so every time you use it, you're using the latest version available.  

For backwards compatibility and emergencies, you can also specify the version of a tool, and use any old version in your trusted repository.  The default, however will be to get and use the latest.

If the trusted repo is offline, or unavailable, you can choose to degrade gracefully to using tools already downloaded.  

You can also choose to limit where your tools can run.  It's all up to you.  DBT is a framework, and frameworks are all about *enablement*. 

# Security

DBT is as secure as the repository you trust to hold the binaries, and the security with which you protect the binary signing keys.  It will ensure, come hell or high water that every bit of the binary downloaded is what it aught to be, and that the signature is one you've decided to trust.  If it can't do that, it'll stop- immediately and scream bloody murder.  

You can make the repo wide open, and give everyone a copy of a non-encrypted key and it'll work.  It's just not recommended.  I just build the tools.  You choose how to use them.

*"If you aim the gun at your foot and pull the trigger, it's UNIX's job to 
ensure reliable delivery of the bullet to where you aimed the gun (in
this case, Mr. Foot)."* -- Terry Lambert, FreeBSD-Hackers mailing list.

# Repository Support

The initial versions of DBT are targeted at the [Artifactory Open Source](https://www.jfrog.com/open-source) repo.  Any sort of WebDAV server that supports authenticated REST should work fine though.
