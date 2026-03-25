#!/usr/bin/env python3
# internal/adapters/scanners/policy_scanner.py

# cmd/policycheck/policy_scanner.py

"""
Python policy scanner for policycheck.
This script extracts function quality facts (LOC, complexity) from Python source files.
Supports both one-shot execution and a persistent worker mode via --worker.
"""

import argparse
import ast
import json
import os
import sys
import threading
import time
from typing import cast, override


FACT_KIND = "function_quality_fact"
IDLE_TIMEOUT = 30.0  # 30 seconds


class PolicyVisitor(ast.NodeVisitor):
    """
    AST visitor that identifies functions and methods and emits quality facts.
    """
    def __init__(self, file_path: str) -> None:
        """
        Initializes the visitor with the file path being scanned.
        """
        self.file_path = file_path
        self.scope_types: list[str] = []

    def emit(self, node: ast.FunctionDef | ast.AsyncFunctionDef, symbol_kind: str) -> None:
        """
        Calculates quality metrics for a function and prints the fact as JSON.
        """
        end_line = getattr(node, "end_lineno", node.lineno)
        param_count = count_parameters(node)

        fact = {
            "kind": FACT_KIND,
            "language": "python",
            "file_path": self.file_path,
            "symbol_name": node.name,
            "line_number": node.lineno,
            "end_line": end_line,
            "complexity": calculate_complexity(node),
            "param_count": param_count,
            "symbol_kind": symbol_kind,
        }
        sys.stdout.write(json.dumps(fact) + "\n")

    def visit_ClassDef(self, node: ast.ClassDef) -> None:
        """
        Tracks class scope to distinguish between functions and methods.
        """
        self.scope_types.append("class")
        self.generic_visit(node)
        _ = self.scope_types.pop()

    def _visit_function(self, node: ast.FunctionDef | ast.AsyncFunctionDef) -> None:
        """
        Common logic for visiting synchronous and asynchronous functions.
        """
        symbol_kind = "function"
        if self.scope_types and self.scope_types[-1] == "class":
            symbol_kind = "method"

        self.emit(node, symbol_kind)
        self.scope_types.append("function")
        self.generic_visit(node)
        _ = self.scope_types.pop()

    @override
    def visit_FunctionDef(self, node: ast.FunctionDef) -> None:
        """
        Visits a synchronous function definition.
        """
        self._visit_function(node)

    @override
    def visit_AsyncFunctionDef(self, node: ast.AsyncFunctionDef) -> None:
        """
        Visits an asynchronous function definition.
        """
        self._visit_function(node)


class ComplexityVisitor(ast.NodeVisitor):
    """
    AST visitor that calculates cyclomatic complexity for a function.
    """
    def __init__(self) -> None:
        """
        Initializes the visitor with a base complexity of 1.
        """
        self.complexity = 1

    def visit_If(self, node: ast.If) -> None:
        """
        Increments complexity for if statements.
        """
        self.complexity += 1
        self.generic_visit(node)

    def visit_For(self, node: ast.For) -> None:
        """
        Increments complexity for for loops.
        """
        self.complexity += 1
        self.generic_visit(node)

    def visit_AsyncFor(self, node: ast.AsyncFor) -> None:
        """
        Increments complexity for async for loops.
        """
        self.complexity += 1
        self.generic_visit(node)

    def visit_While(self, node: ast.While) -> None:
        """
        Increments complexity for while loops.
        """
        self.complexity += 1
        self.generic_visit(node)

    def visit_ExceptHandler(self, node: ast.ExceptHandler) -> None:
        """
        Increments complexity for exception handlers.
        """
        self.complexity += 1
        self.generic_visit(node)

    def visit_Match(self, node: ast.Match) -> None:
        """
        Increments complexity for match statements based on the number of cases.
        """
        self.complexity += len(node.cases)
        self.generic_visit(node)

    def visit_BoolOp(self, node: ast.BoolOp) -> None:
        """
        Increments complexity for boolean operations (and/or).
        """
        self.complexity += max(0, len(node.values) - 1)
        self.generic_visit(node)


def calculate_complexity(node: ast.FunctionDef | ast.AsyncFunctionDef) -> int:
    """
    Calculates the cyclomatic complexity of a function body.
    """
    visitor = ComplexityVisitor()
    for statement in node.body:
        visitor.visit(statement)
    return visitor.complexity


def count_parameters(node: ast.FunctionDef | ast.AsyncFunctionDef) -> int:
    """
    Counts positional-only, positional, keyword-only, and variadic parameters.
    """
    param_count = len(node.args.posonlyargs) + len(node.args.args) + len(node.args.kwonlyargs)
    if node.args.vararg:
        param_count += 1
    if node.args.kwarg:
        param_count += 1

    return param_count


def process_file(args_file: str, args_root: str) -> None:
    """
    Processes a single Python file.
    """
    try:
        rel_path = os.path.relpath(args_file, args_root)
    except ValueError:
        rel_path = args_file
    rel_path = rel_path.replace(os.sep, "/")

    try:
        with open(args_file, "r", encoding="utf-8") as handle:
            source = handle.read()
    except OSError as err:
        sys.stderr.write(f"Failed to read file {args_file}: {err}\n")
        return

    try:
        tree = ast.parse(source, filename=args_file)
    except SyntaxError as err:
        sys.stderr.write(f"SyntaxError parsing {args_file}: {err}\n")
        return
    except Exception as err:
        sys.stderr.write(f"Error parsing {args_file}: {err}\n")
        return

    visitor = PolicyVisitor(rel_path)
    visitor.visit(tree)


def run_worker() -> None:
    """
    Runs the scanner in worker mode, reading commands from stdin.
    """
    last_active = time.time()

    def timeout_checker() -> None:
        nonlocal last_active
        while True:
            time.sleep(1)
            if time.time() - last_active > IDLE_TIMEOUT:
                os._exit(0)

    thread = threading.Thread(target=timeout_checker, daemon=True)
    thread.start()

    for line in sys.stdin:
        trimmed = line.strip()
        if not trimmed:
            continue

        last_active = time.time()

        try:
            request = json.loads(trimmed)
            command = request.get("command")
            if command == "scan":
                files = request.get("files", [])
                root = request.get("root")
                if files and root:
                    for f in files:
                        process_file(f, root)
                    sys.stdout.write(json.dumps({"kind": "scan_complete"}) + "\n")
                    sys.stdout.flush()
            elif command == "exit":
                break
        except Exception as err:
            sys.stderr.write(f"Worker error: {err}\n")

    sys.exit(0)


def main() -> None:
    """
    Main entry point for the Python policy scanner.
    """
    parser = argparse.ArgumentParser(description="Python policy scanner for policycheck")
    _ = parser.add_argument("--file", nargs="+", help="Files to scan")
    _ = parser.add_argument("--root", help="Project root directory")
    _ = parser.add_argument("--worker", action="store_true", help="Run in persistent worker mode")
    args = parser.parse_args()

    if args.worker:
        run_worker()
        return

    if not args.file or not args.root:
        parser.print_help()
        sys.exit(1)

    args_files = cast(list[str], args.file)
    args_root = cast(str, args.root)

    for args_file in args_files:
        process_file(args_file, args_root)


if __name__ == "__main__":
    main()
