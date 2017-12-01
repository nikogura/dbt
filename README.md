# DBT  *Distributed Binary Toolkit*

A framework for running signed binary tools from a central, trusted repository.

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

DBT is as secure as the repository you trust to hold the binaries, and the security with which you protect the binary signing keys.

You can make the repo wide open, and give everyone a copy of a non-encrypted key and it'll work.  It's just not recommended.

# Repository Support

The initial versions of DBT are targeted at the [Artifactory Open Source](https://www.jfrog.com/open-source) repo.  Any sort of WebDAV server that supports authenticated REST should work fine though.
