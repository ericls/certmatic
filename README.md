# Certmatic

## Motivation and goals
Certmatic is born out of frustration after repeatedly implementing custom domain support for different SaaS applications.

This library aims to provide a "managed" experience for the following common tasks related to custom domains:
- On demand SSL certificate issuance and renewal
- Domain ownership verification
- Tools to guide users through the custom domain setup process
- SSL termination (as Caddy plugin)

Certmatic aims to be composable.
In a typical setup, a SaaS application will use Certmatic as a Caddy plugin to handle SSL termination for custom domains,
and use Certmatic's API(SDK?) to manage domain ownership verification and certificate issuance/renewal in their backend.
However, Certmatic also aim to be able to act as a certification manager alone without SSL termination, if the SaaS application has its own way to terminate SSL, for example, using another ingress controller or a CDN.


## Current status
Nothing is implemented yet :).
