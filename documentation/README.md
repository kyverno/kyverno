# CNCF Hugo Starter

This repository contains a boilerplate static site generator setup for creating CNCF documentation projects. We strongly recommend using this setup (it helps us help you and your project!), but none of the technologies in the stack are strictly required.

The starter uses the following:
* **[Hugo](https://gohugo.io/)** as a static site generator
* **[Bulma](https://bulma.io/)** as a CSS framework
* **[Netlify](https://www.netlify.com/)** for building, hosting, and DNS management

## Running locally

Make sure you have [npm](https://www.npmjs.com/) and [yarn](https://yarnpkg.com/) installed. Clone this repository and run the following two commands in its directory:

```shell
# Install npm assets (just Bulma for Sass/CSS)
yarn

# Run the server locally
make serve
```

## Running on Netlify

Netlify is a CI/CD build tool and hosting solution for (among other things) static sites. We **strongly** recommend using Netlify unless you have a good reason not to.

This repository comes with a pre-configured [`netlify.toml`](https://github.com/cncf/hugo-netlify-starter/blob/master/netlify.toml) file. To build to Netlify:

1. Go to [netlify.com](https://netlify.com) and sign up. We recommend signing up using a GitHub account.
2. Click **New Site from Git**, and give Netlify access to your GitHub account.
  > **Note:** For projects with lots of contributors, it can be handy to create a general/bot account instead of granting access with a personal account.

3. Install Netlify with access to your documentation site repository.
4. Leave all other settings as default and click **Deploy Site**.

# What's included

This repository has two layouts with minimal styling, all stored under `/layouts/_default`:

* A **homepage** template, a basic homepage which uses the following:
  * The `index.html` file and partials in the `/partials/home` directory
  * Some helpers in the `/assets/sass/helpers.sass` file
* A **docs** template, a basic content page with submenu which uses the following:
  * The `single.html` file and partials in the `/partials/docs` directory
  * Classes in the `/assets/sass/helpers.sass` and `/assets/sass/_docs.sass` files

Both use default components and styling from the Bulma CSS framework. No menus are structured, because menu structure is highly dependant on the nature of the project.
