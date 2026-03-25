// cmd/policycheck/policy_scanner.ts

/**
 * TypeScript policy scanner for ISR.
 * This script extracts function quality facts (LOC, complexity) from TypeScript source files.
 * Supports both one-shot execution and a persistent worker mode via --worker.
 */

import * as fs from 'node:fs';
import * as path from 'node:path';
import * as ts from 'typescript';
import * as readline from 'node:readline';

const FACT_KIND = 'function_quality_fact';
const IDLE_TIMEOUT = 30000; // 30 seconds

type PolicyFact = {
    kind: string;
    language: string;
    path: string;
    name: string;
    line: number;
    end_line: number;
    loc: number;
    ctx: number;
    symbol_kind: string;
};

/**
 * Returns a normalized relative path from the root to the file.
 */
function relativePolicyPath(rootPath: string, filePath: string): string {
    return path.relative(rootPath, filePath).split(path.sep).join('/');
}

/**
 * Emits a quality fact for a symbol as a JSON line.
 */
function emitFact(sourceFile: ts.SourceFile, relPath: string, name: string, symbolKind: string, node: ts.Node): void {
    const start = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile)).line + 1;
    const endLine = sourceFile.getLineAndCharacterOfPosition(node.getEnd()).line + 1;
    const fact: PolicyFact = {
        kind: FACT_KIND,
        language: 'typescript',
        path: relPath,
        name,
        line: start,
        end_line: endLine,
        loc: Math.max(1, endLine - start + 1),
        ctx: calculateComplexity(node),
        symbol_kind: symbolKind,
    };
    process.stdout.write(JSON.stringify(fact) + '\n');
}

/**
 * Calculates the cyclomatic complexity of a function body or expression.
 */
function calculateComplexity(node: ts.Node): number {
    let complexity = 1;

    function isBranchPoint(current: ts.Node): boolean {
        if (
            ts.isIfStatement(current) ||
            ts.isForStatement(current) ||
            ts.isForInStatement(current) ||
            ts.isForOfStatement(current) ||
            ts.isWhileStatement(current) ||
            ts.isDoStatement(current) ||
            ts.isCaseClause(current) ||
            ts.isCatchClause(current) ||
            ts.isConditionalExpression(current)
        ) {
            return true;
        }
        return (
            ts.isBinaryExpression(current) &&
            (current.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken ||
                current.operatorToken.kind === ts.SyntaxKind.BarBarToken)
        );
    }

    function visit(current: ts.Node): void {
        if (isBranchPoint(current)) {
            complexity += 1;
        }
        ts.forEachChild(current, visit);
    }

    const body = getFunctionBody(node);
    if (body) {
        ts.forEachChild(body, visit);
    }

    return complexity;
}

/**
 * Retrieves the executable body of a function-like node.
 */
function getFunctionBody(node: ts.Node): ts.Node | undefined {
    if (
        ts.isFunctionDeclaration(node) ||
        ts.isMethodDeclaration(node) ||
        ts.isConstructorDeclaration(node) ||
        ts.isFunctionExpression(node) ||
        ts.isArrowFunction(node)
    ) {
        return node.body;
    }

    return undefined;
}

/**
 * Identifies the name of a function declared via variable assignment.
 */
function getVariableFunctionName(node: ts.VariableDeclaration): string | undefined {
    if (!ts.isIdentifier(node.name) || !node.initializer) {
        return undefined;
    }
    if (!ts.isArrowFunction(node.initializer) && !ts.isFunctionExpression(node.initializer)) {
        return undefined;
    }
    return node.name.text;
}

/**
 * Recursively scans a source file for function and method declarations.
 */
function scanSource(sourceFile: ts.SourceFile, relPath: string): void {
    const classStack: string[] = [];

    function visit(node: ts.Node): void {
        if (ts.isClassDeclaration(node) && node.name) {
            classStack.push(node.name.text);
            ts.forEachChild(node, visit);
            classStack.pop();
            return;
        }

        if (ts.isFunctionDeclaration(node) && node.name) {
            emitFact(sourceFile, relPath, node.name.text, 'function', node);
        } else if (ts.isMethodDeclaration(node) && ts.isIdentifier(node.name)) {
            emitFact(sourceFile, relPath, node.name.text, 'method', node);
        } else if (ts.isConstructorDeclaration(node)) {
            const owner = classStack[classStack.length - 1] ?? 'constructor';
            emitFact(sourceFile, relPath, owner, 'method', node);
        } else if (ts.isVariableDeclaration(node)) {
            const variableName = getVariableFunctionName(node);
            if (variableName && node.initializer) {
                emitFact(sourceFile, relPath, variableName, 'function', node.initializer);
            }
        }

        ts.forEachChild(node, visit);
    }

    visit(sourceFile);
}

/**
 * Processes a single file.
 */
function processFile(filePath: string, rootPath: string): void {
    let sourceText = '';
    const absolutePath = path.resolve(filePath);
    try {
        sourceText = fs.readFileSync(absolutePath, 'utf8');
    } catch (err) {
        process.stderr.write(`Failed to read file ${absolutePath}: ${err}\n`);
        return;
    }

    const sourceFile = ts.createSourceFile(absolutePath, sourceText, ts.ScriptTarget.Latest, true);
    scanSource(sourceFile, relativePolicyPath(rootPath, absolutePath));
}

/**
 * Runs the scanner in worker mode, reading commands from stdin.
 */
function runWorker(): void {
    const rl = readline.createInterface({
        input: process.stdin,
        terminal: false,
    });

    let idleTimer: NodeJS.Timeout | null = null;

    const resetIdleTimer = () => {
        if (idleTimer) clearTimeout(idleTimer);
        idleTimer = setTimeout(() => {
            process.exit(0);
        }, IDLE_TIMEOUT);
    };

    resetIdleTimer();

    rl.on('line', (line) => {
        const trimmed = line.trim();
        if (!trimmed) return;

        resetIdleTimer();

        try {
            const request = JSON.parse(trimmed);
            if (request.command === 'scan' && Array.isArray(request.files) && request.root) {
                for (const file of request.files) {
                    processFile(file, request.root);
                }
                // Signal completion of this batch
                process.stdout.write(JSON.stringify({ kind: 'scan_complete' }) + '\n');
            } else if (request.command === 'exit') {
                process.exit(0);
            }
        } catch (err) {
            process.stderr.write(`Worker error processing line: ${err}\n`);
        }
    });

    // Ensure we exit if stdin closes
    rl.on('close', () => {
        process.exit(0);
    });
}

/**
 * Main entry point for the TypeScript policy scanner.
 */
function main(): void {
    const args = process.argv.slice(2);
    
    if (args.includes('--worker')) {
        runWorker();
        return;
    }

    const filePaths: string[] = [];
    let rootPath = '';

    for (let i = 0; i < args.length; i++) {
        if (args[i] === '--file') {
            for (let j = i + 1; j < args.length; j++) {
                const arg = args[j];
                if (arg === undefined || arg.startsWith('--')) break;
                filePaths.push(arg);
            }
        } else if (args[i] === '--root') {
            const next = args[i + 1];
            rootPath = next === undefined ? '' : path.resolve(next);
            i++;
        }
    }

    if (filePaths.length === 0 || rootPath === '') {
        process.stderr.write('Usage: node policy_scanner.cjs [--worker] | [--file <path1> ... --root <path>]\n');
        process.exit(1);
    }

    for (const filePath of filePaths) {
        processFile(filePath, rootPath);
    }
}

main();
