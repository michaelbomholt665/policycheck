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
	params: string[];
	symbol_kind: string;
	docstring: string;
};

type CommentRange = {
	kind: ts.CommentKind;
	text: string;
};

export type ScanContext = {
	sourceFile: ts.SourceFile;
	relPath: string;
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
function emitFact(ctx: ScanContext, name: string, symbolKind: string, node: ts.Node): void {
	const start = ctx.sourceFile.getLineAndCharacterOfPosition(node.getStart(ctx.sourceFile)).line + 1;
	const endLine = ctx.sourceFile.getLineAndCharacterOfPosition(node.getEnd()).line + 1;

	let paramCount = 0;
	let params: string[] = [];
	let docstring = '';

	if (isCountedFunctionLike(node)) {
		paramCount = node.parameters.length;
		params = node.parameters.map((p) => p.name.getText(ctx.sourceFile));
		docstring = extractLeadingDocumentation(ctx.sourceFile, node);
	}

	const fact: PolicyFact = {
		kind: FACT_KIND,
		language: 'typescript',
		file_path: ctx.relPath,
		symbol_name: name,
		line_number: start,
		end_line: endLine,
		complexity: calculateComplexity(node),
		param_count: paramCount,
		params: params,
		symbol_kind: symbolKind,
		docstring: docstring,
	};
	process.stdout.write(JSON.stringify(fact) + '\n');
}

/**
 * Returns attached leading comments for a node when they are directly adjacent.
 */
function leadingCommentRanges(sourceFile: ts.SourceFile, node: ts.Node): CommentRange[] {
	const ranges = ts.getLeadingCommentRanges(sourceFile.text, node.getFullStart()) ?? [];
	if (ranges.length === 0) {
		return [];
	}

	const attached: CommentRange[] = [];
	let expectedEnd = node.getStart(sourceFile);
	for (let i = ranges.length - 1; i >= 0; i -= 1) {
		const range = ranges[i];
		if (!range) {
			continue;
		}
		const between = sourceFile.text.slice(range.end, expectedEnd);
		if (!isDirectlyAttachedCommentGap(between)) {
			break;
		}

		attached.unshift({
			kind: range.kind,
			text: sourceFile.text.slice(range.pos, range.end),
		});
		expectedEnd = range.pos;
	}

	return attached;
}

/**
 * Reports whether the gap between a comment and node is small enough to count as attached.
 */
function isDirectlyAttachedCommentGap(gap: string): boolean {
	if (gap.trim() !== '') {
		return false;
	}

	const newlineCount = (gap.match(/\n/g) ?? []).length;
	return newlineCount <= 1;
}

/**
 * Extracts the directly attached documentation comment text for a node.
 */
function extractLeadingDocumentation(sourceFile: ts.SourceFile, node: ts.Node): string {
	const comments = leadingCommentRanges(sourceFile, node);
	if (comments.length === 0) {
		return '';
	}

	return comments.map((comment) => comment.text).join('\n');
}

const branchKinds = new Set([
	ts.SyntaxKind.IfStatement,
	ts.SyntaxKind.ForStatement,
	ts.SyntaxKind.ForInStatement,
	ts.SyntaxKind.ForOfStatement,
	ts.SyntaxKind.WhileStatement,
	ts.SyntaxKind.DoStatement,
	ts.SyntaxKind.CatchClause,
	ts.SyntaxKind.ConditionalExpression,
	ts.SyntaxKind.CaseClause,
]);

/**
 * Returns the number of complexity points added by one branch node.
 */
function branchComplexity(node: ts.Node): number {
	if (branchKinds.has(node.kind)) {
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
function emitNodeFact(ctx: ScanContext, classStack: string[], node: ts.Node): void {
	if (ts.isFunctionDeclaration(node) && node.name) {
		emitFact(ctx, node.name.text, 'function', node);
		return;
	}

	if (ts.isMethodDeclaration(node) && ts.isIdentifier(node.name)) {
		emitFact(ctx, node.name.text, 'method', node);
		return;
	}

	if (ts.isConstructorDeclaration(node)) {
		const owner = classStack.length > 0 ? (classStack.at(-1) ?? 'constructor') : 'constructor';
		emitFact(ctx, owner, 'method', node);
		return;
	}

	if (ts.isVariableDeclaration(node)) {
		const variableName = getVariableFunctionName(node);
		if (variableName !== undefined && node.initializer) {
			emitFact(ctx, variableName, 'function', node.initializer);
		}
	}
}

/**
 * Visits a source tree and emits scanner facts.
 */
function visitScanNode(ctx: ScanContext, classStack: string[], node: ts.Node): void {
	if (ts.isClassDeclaration(node) && node.name) {
		classStack.push(node.name.text);
		ts.forEachChild(node, (child) => visitScanNode(ctx, classStack, child));
		classStack.pop();
		return;
	}

	emitNodeFact(ctx, classStack, node);
	ts.forEachChild(node, (child) => visitScanNode(ctx, classStack, child));
}

/**
 * Recursively scans a source file for function and method declarations.
 */
function scanSource(sourceFile: ts.SourceFile, relPath: string): void {
	const classStack: string[] = [];
	visitScanNode({ sourceFile, relPath }, classStack, sourceFile);
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
