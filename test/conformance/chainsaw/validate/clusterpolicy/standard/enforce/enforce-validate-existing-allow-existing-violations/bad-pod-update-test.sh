if kubectl label po badpod-allow-existing foo=bad1 --overwrite 2>&1 | grep -q  "validation error: rule check-labels" 
then 
  echo "Test succeed, updating violating preexisting resource does throw error"
  exit 0
else 
  echo "Test failed, updating violating preexisting resource should throw error"
  exit 1
fi
