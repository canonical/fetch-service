import datetime

project = "Fetch Service"
author = "Canonical Group Ltd"
html_title = project + " documentation"
copyright = "%s, %s" % (datetime.date.today().year, author)

ogp_site_url = "https://canonical-fetch-service.readthedocs-hosted.com/"
ogp_site_name = project
ogp_image = "https://assets.ubuntu.com/v1/253da317-image-document-ubuntudocs.svg"

html_context = {
    "product_page": "github.com/canonical/fetch-service",
    "github_url": "https://github.com/canonical/fetch-service",
}

# Add extensions
extensions = [
    "canonical_sphinx",
    "sphinxext.rediraffe",
    "sphinx_related_links",
]

github_username = "canonical"
github_repository = "fetch-service"

# Client-side page redirects.
rediraffe_redirects = "redirects.txt"
