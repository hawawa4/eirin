This project is a Golang project that aims to create a simple but powerful library manager for astrophotography. 
The project uses Go for the backend part, which in turn will use Siril for the ACTUAL astro processing, with Wails as a bridge to use Svelte for the frontend display part.
The Svelte frontend uses typescript with prettier and eslint.

The main features of the application are:

- Multiplatform
- Local, classic desktop app, no webserver
- "NAS first" approach: Your astrophotography data lives in the NAS, you handle Siril projects through symlinks on local folders
- Copy data from the Seestar onto the NAS without duplicating files (uses RSync)
- Preview autostretched FITS and delete them from the NAS/project
- Automatically handle sequence creation and management, offloading the processing to siril via cli

## Development notes

We are currently in an unreleased version. Any data in the SQLite database does NOT need to be migrated and it's perfectly fine to assume we can throw it away. Making the code simpler and not dealing with sequential migrations for now is better.