#!/bin/bash

# Kyverno CLI Completion Feature Quick Demo Script
# This script demonstrates the completion feature step by step

set -e

echo "======================================"
echo "  Kyverno CLI Completion Demo"
echo "======================================"
echo

# Check if binary exists
if [ ! -f "./cmd/cli/kubectl-kyverno/kubectl-kyverno" ]; then
    echo "❌ CLI binary not found. Building now..."
    make build-cli
else
    echo "✅ CLI binary found"
fi

echo
echo "🎯 Demo Step 1: Show completion command in help"
echo "----------------------------------------------"
./cmd/cli/kubectl-kyverno/kubectl-kyverno --help | grep -A 10 "Available Commands:"

echo
echo "🎯 Demo Step 2: Show completion command help"
echo "--------------------------------------------"
./cmd/cli/kubectl-kyverno/kubectl-kyverno completion --help | head -20

echo
echo "🎯 Demo Step 3: Test all shell completion generation"
echo "---------------------------------------------------"

# Test bash completion
echo "Testing bash completion..."
./cmd/cli/kubectl-kyverno/kubectl-kyverno completion bash > /tmp/demo-bash-completion.sh
echo "✅ Bash completion generated ($(wc -l < /tmp/demo-bash-completion.sh) lines)"

# Test zsh completion  
echo "Testing zsh completion..."
./cmd/cli/kubectl-kyverno/kubectl-kyverno completion zsh > /tmp/demo-zsh-completion.sh
echo "✅ Zsh completion generated ($(wc -l < /tmp/demo-zsh-completion.sh) lines)"

# Test fish completion
echo "Testing fish completion..."
./cmd/cli/kubectl-kyverno/kubectl-kyverno completion fish > /tmp/demo-fish-completion.fish
echo "✅ Fish completion generated ($(wc -l < /tmp/demo-fish-completion.fish) lines)"

# Test powershell completion
echo "Testing PowerShell completion..."
./cmd/cli/kubectl-kyverno/kubectl-kyverno completion powershell > /tmp/demo-powershell-completion.ps1
echo "✅ PowerShell completion generated ($(wc -l < /tmp/demo-powershell-completion.ps1) lines)"

echo
echo "🎯 Demo Step 4: Show completion script samples"
echo "----------------------------------------------"
echo "First 10 lines of bash completion script:"
head -10 /tmp/demo-bash-completion.sh

echo
echo "First 10 lines of zsh completion script:"  
head -10 /tmp/demo-zsh-completion.sh

echo
echo "🎯 Demo Step 5: Test completion installation (temporary)"
echo "-------------------------------------------------------"
echo "Loading bash completion for current session..."

# Create alias for easier demonstration
alias kyverno="./cmd/cli/kubectl-kyverno/kubectl-kyverno"

# Source the completion
if source /tmp/demo-bash-completion.sh 2>/dev/null; then
    echo "✅ Bash completion loaded successfully!"
    echo
    echo "💡 Now you can test tab completion manually:"
    echo "   - Type 'kyverno ' and press TAB to see all commands"
    echo "   - Type 'kyverno completion ' and press TAB to see shell options"
    echo "   - Type 'kyverno apply --' and press TAB to see flags"
else
    echo "⚠️  Completion loading skipped (may require interactive shell)"
fi

echo
echo "🎯 Demo Step 6: Verify command structure supports completion"
echo "-----------------------------------------------------------"
echo "Main commands that support completion:"
./cmd/cli/kubectl-kyverno/kubectl-kyverno --help | grep -E "^\s+\w+" | head -10

echo
echo "Create subcommands that support completion:"
./cmd/cli/kubectl-kyverno/kubectl-kyverno create --help | grep -E "^\s+\w+" | head -10

echo
echo "🧹 Cleaning up demo files..."
rm -f /tmp/demo-*.sh /tmp/demo-*.fish /tmp/demo-*.ps1

echo
echo "✅ Demo completed successfully!"
echo "📖 See DEMO_CLI_COMPLETION.md for detailed instructions"
echo "🚀 The completion feature is ready for use!"

