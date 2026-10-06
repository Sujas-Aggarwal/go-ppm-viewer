# A Very Basic SDL2 Dependent PPM Viewer Written in Go
## Usage - 
1. Install Dependencies
```bash
go mod tidy 
```
2. Build
```bash
go build ./cmd/ppm-viewer   
```
3. Use (PPM file must be using P6 Format (binary format))
```bash
./ppm-viewer ${FILE_PATH}
```