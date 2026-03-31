package main

import (
"fmt"
"os"
)

func main() {
fmt.Println("fugue api server")
_ = os.Getenv("PORT")
}
