This project is a Golang project that aims to create a simple but powerful library manager for astrophotography. 
The project uses Go with Wails for the frontend.

The main features of the application are:

- Multiplatform
- Local, classic desktop app, no webserver
- "NAS first" approach: Your astrophotography data lives in the NAS, you handle Siril projects through symlinks on local folders
- Copy data from the Seestar onto the NAS without duplicating files (uses RSync)
- Preview autostretched FITS and delete them from the NAS/project
- Automatically handle sequence creation and management, offloading the processing to siril via cli