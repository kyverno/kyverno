# Kyverno Web Site

This folder contains the https://kyverno.io website.

The site is published in the gh_pages branch. To build the site:

1. Clone the Hugo template:

````bash
cd themes
 git clone https://github.com/nirmata/github-project-landing-page
 ````

 2. Make changes as needed. Then publish using:

````bash
publish-to-gh-pages.sh
````

To build and test locally, install and run hugo:

````bash
hugo server
````
