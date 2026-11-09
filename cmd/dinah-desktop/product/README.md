# Product files

This directory is where a build of another product puts the page files that are its own: `product-mark.svg`, the mark the page shows beside the product's name; `brand.css`, the brand stylesheet; `theme-data.js`, the built-in themes, one of which the product's profile names as its default; and any other page file the product means to replace, under the same path it has in `frontend`. The window serves a path found here from here, and any other from `frontend` (`product_assets.go`).

Dinah's own build ships this directory with this file alone, and the page is Dinah's. The product's name, folders, ports and the agent providers it offers come from its profile, which its build injects at link time (`internal/profile/injected.go`). The design is [docs/design/product-definition.md](../../../docs/design/product-definition.md).
