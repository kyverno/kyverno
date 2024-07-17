if kubectl label po goodpod foo=bad1 --overwrite 2>&1 | grep -q  "validation error: rule check-labels" 
then 
  echo "Test succeed, updating violating resource throws error"
  exit 0
else 
  echo "Test failed, updating violating resource did not throw error"
  exit 1
fi
