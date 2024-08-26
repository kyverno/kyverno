if kubectl label po badpod foo=bad1 --overwrite 2>&1 | grep -q  "validation error: rule check-labels" 
then 
  echo "Test failed, updating violating preexisting resource should not throw error"
  exit 1
else 
  echo "Test succeed, updating violating preexisting resource does not throw error"
  exit 0
fi
