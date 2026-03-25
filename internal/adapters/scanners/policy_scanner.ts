// cmd/policycheck/policy_scanner.ts

/**
 * TypeScript policy scanner for policycheck.
 * This script extracts function quality facts (LOC, complexity) from TypeScript source files.
 * Supports both one-shot execution and a persistent worker mode via --worker.
 */

import * as fs from 'node:fs';
import * as path from 'node:path';
import * as readline from 'node:readline';
import * as ts from 'typescript';

const FACT_KIND = 'function_quality_fact';
const IDLE_TIMEOUT = 30000; // 30 seconds

type PolicyFact = {
	kind: string;
	language: string;
	file_path: string;
	symbol_name: string;
	line_number: number;
	end_line: number;
	complexity: number;
	param_count: number;
	symbol_kind: string;
};

type WorkerRequest =
	| { command: 'scan'; files: string[]; root: string }
	| { command: 'exit' };

/**
 * Returns a normalized relative path from the root to the file.
 */
function relativePolicyPath(rootPath: string, filePath: string): string {
	return path.relative(rootPath, filePath).split(path.sep).join('/');
}

/**
 * Reports whether a node is a function-like declaration handled by this scanner.
 */
function isCountedFunctionLike(node: ts.Node): node is ts.FunctionDeclaration | ts.MethodDeclaration | ts.ConstructorDeclaration | ts.FunctionExpression | ts.ArrowFunction {
	return ts.isFunctionDeclaration(node) || ts.isMethodDeclaration(node) || ts.isConstructorDeclaration(node) || ts.isFunctionExpression(node) || ts.isArrowFunction(node);
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
		file_path: relPath,
		symbol_name: name,
		line_number: start,
		end_line: endLine,
		complexity: calculateComplexity(node),
		param_count: isCountedFunctionLike(node) ? node.parameters.length : 0,
		symbol_kind: symbolKind,
	};
	process.stdout.write(JSON.stringify(fact) + '\n');
}

/**
 * Returns the number of complexity points added by one branch node.
 */
function branchComplexity(node: ts.Node): number {
	if (
		ts.isIfStatement(node) ||
		ts.isForStatement(node) ||
		ts.isForInStatement(node) ||
		ts.isForOfStatement(node) ||
		ts.isWhileStatement(node) ||
		ts.isDoStatement(node) ||
		ts.isCatchClause(node) ||
		ts.isConditionalExpression(node)
	) {
		return 1;
	}

	if (ts.isCaseClause(node)) {
		return 1;
	}

	if (
		ts.isBinaryExpression(node) &&
		(node.operatorToken.kind === ts.SyntaxKind.AmpersandAmpersandToken ||
			node.operatorToken.kind === ts.SyntaxKind.BarBarToken)
	) {
		return 1;
	}

	return 0;
}

/**
 * Walks a node tree and increments complexity for each branch point.
 */
function visitComplexityNode(node: ts.Node, apply: (delta: number) => void): void {
	const delta = branchComplexity(node);
	if (delta > 0) {
		apply(delta);
	}

	ts.forEachChild(node, (child) => visitComplexityNode(child, apply));
}

/**
 * Calculates the cyclomatic complexity of a function body or expression.
 */
function calculateComplexity(node: ts.Node): number {
	let complexity = 1;
	const body = getFunctionBody(node);
	if (body) {
		ts.forEachChild(body, (child) => visitComplexityNode(child, (delta) => {
			complexity += delta;
		}));
	}

	return complexity;
}

/**
 * Retrieves the executable body of a function-like node.
 */
function getFunctionBody(node: ts.Node): ts.Node | undefined {
	if (isCountedFunctionLike(node)) {
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
 * Emits facts for one scan node when it represents a supported symbol.
 */
function emitNodeFact(sourceFile: ts.SourceFile, relPath: string, classStack: string[], node: ts.Node): void {
	if (ts.isFunctionDeclaration(node) && node.name) {
		emitFact(sourceFile, relPath, node.name.text, 'function', node);
		return;
	}

	if (ts.isMethodDeclaration(node) && ts.isIdentifier(node.name)) {
		emitFact(sourceFile, relPath, node.name.text, 'method', node);
		return;
	}

	if (ts.isConstructorDeclaration(node)) {
		const owner = classStack.length > 0 ? classStack[classStack.length - 1] : 'constructor';
		emitFact(sourceFile, relPath, owner, 'method', node);
		return;
	}

	if (ts.isVariableDeclaration(node)) {
		const variableName = getVariableFunctionName(node);
		if (variableName && node.initializer) {
			emitFact(sourceFile, relPath, variableName, 'function', node.initializer);
		}
	}
}

/**
 * Visits a source tree and emits scanner facts.
 */
function visitScanNode(sourceFile: ts.SourceFile, relPath: string, classStack: string[], node: ts.Node): void {
	if (ts.isClassDeclaration(node) && node.name) {
		classStack.push(node.name.text);
		ts.forEachChild(node, (child) => visitScanNode(sourceFile, relPath, classStack, child));
		classStack.pop();
		return;
	}

	emitNodeFact(sourceFile, relPath, classStack, node);
	ts.forEachChild(node, (child) => visitScanNode(sourceFile, relPath, classStack, child));
}

/**
 * Recursively scans a source file for function and method declarations.
 */
function scanSource(sourceFile: ts.SourceFile, relPath: string): void {
	const classStack: string[] = [];
	visitScanNode(sourceFile, relPath, classStack, sourceFile);
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
 * Resets the worker idle timer.
 */
function resetIdleTimer(currentTimer: ReturnType<typeof setTimeout> | null): ReturnType<typeof setTimeout> {
	if (currentTimer) {
		clearTimeout(currentTimer);
	}

	return setTimeout(() => {
		process.exit(0);
	}, IDLE_TIMEOUT);
}

/**
 * Parses one worker request line.
 */
function parseWorkerRequest(line: string): WorkerRequest {
	return JSON.parse(line) as WorkerRequest;
}

/**
 * Handles one worker request.
 */
function handleWorkerRequest(request: WorkerRequest): void {
	if (request.command === 'exit') {
		process.exit(0);
	}

	for (const file of request.files) {
		processFile(file, request.root);
	}
	process.stdout.write(JSON.stringify({ kind: 'scan_complete' }) + '\n');
}

/**
 * Runs the scanner in worker mode, reading commands from stdin.
 */
function runWorker(): void {
	const rl = readline.createInterface({
		input: process.stdin,
		terminal: false,
	});

	let idleTimer: ReturnType<typeof setTimeout> | null = null;
	idleTimer = resetIdleTimer(idleTimer);

	rl.on('line', (line: string) => {
		const trimmed = line.trim();
		if (!trimmed) {
			return;
		}

		idleTimer = resetIdleTimer(idleTimer);

		try {
			handleWorkerRequest(parseWorkerRequest(trimmed));
		} catch (err) {
			process.stderr.write(`Worker error processing line: ${err}\n`);
		}
	});

	rl.on('close', () => {
		process.exit(0);
	});
}

type ParsedArgs = {
	filePaths: string[];
	rootPath: string;
};

/**
 * Parses CLI arguments for one-shot scanner mode.
 */
function parseCLIArgs(args: string[]): ParsedArgs {
	const filePaths: string[] = [];
	let rootPath = '';

	for (let i = 0; i < args.length; i += 1) {
		const arg = args[i];
		if (arg === '--file') {
			for (let j = i + 1; j < args.length; j += 1) {
				const fileArg = args[j];
				if (fileArg === undefined || fileArg.startsWith('--')) {
					break;
				}
				filePaths.push(fileArg);
			}
			continue;
		}

		if (arg === '--root') {
			const next = args[i + 1];
			rootPath = next === undefined ? '' : path.resolve(next);
			i += 1;
		}
	}

	return { filePaths, rootPath };
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

	const parsed = parseCLIArgs(args);
	if (parsed.filePaths.length === 0 || parsed.rootPath === '') {
		process.stderr.write('Usage: node policy_scanner.cjs [--worker] | [--file <path1> ... --root <path>]\n');
		process.exit(1);
	}

	for (const filePath of parsed.filePaths) {
		processFile(filePath, parsed.rootPath);
	}
}

main();
