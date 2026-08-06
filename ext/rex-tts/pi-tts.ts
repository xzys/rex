/**
 * pi-tts: speak pi's final answers through the shared rex-tts engine.
 *
 * Mirrors the zot frontend: buffer the latest assistant message, speak it
 * once the agent settles (so tool-use steps stay silent), and keep answers
 * speech-friendly by extending the system prompt. Synthesis happens in a
 * long-lived `rex-tts --serve` child, so the model stays resident and the
 * child dies with pi.
 */
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";
import { spawn, type ChildProcessByStdio } from "node:child_process";
import { createInterface } from "node:readline";
import * as os from "node:os";
import * as path from "node:path";
import type { Readable, Writable } from "node:stream";

const BIN =
	process.env.REX_TTS_BIN ??
	path.join(os.homedir(), ".local", "lib", "rex-tts", "rex-tts");

const SPEECH_PROMPT = `

## Voice output

Your final answer each turn is read aloud by a text-to-speech engine. Keep
final answers speech-friendly: short conversational sentences, plain prose, no
markdown headers, bullet lists, tables, code blocks, or URLs. If code or a diff
is the deliverable, write it to a file with tools and summarize what you did in
one or two spoken sentences.`;

type Engine = ChildProcessByStdio<Writable, Readable, null>;
type Resp = { ok?: boolean; text?: string; error?: string };

function textOf(content: unknown): string {
	if (typeof content === "string") return content;
	if (!Array.isArray(content)) return "";
	const parts: string[] = [];
	for (const block of content) {
		if (
			block &&
			typeof block === "object" &&
			(block as { type?: string }).type === "text" &&
			typeof (block as { text?: string }).text === "string"
		) {
			parts.push((block as { text: string }).text);
		}
	}
	return parts.join("\n");
}

export default function (pi: ExtensionAPI) {
	let child: Engine | null = null;
	let pending: Array<(r: Resp) => void> = [];

	function engine(): Engine | null {
		if (child && child.exitCode === null) return child;
		let spawned: Engine;
		try {
			spawned = spawn(BIN, ["--serve"], {
				stdio: ["pipe", "pipe", "inherit"],
			}) as Engine;
		} catch {
			return null;
		}
		child = spawned;
		const lines = createInterface({ input: spawned.stdout });
		lines.on("line", (line) => {
			const done = pending.shift();
			if (!done) return;
			try {
				done(JSON.parse(line) as Resp);
			} catch {
				done({ error: `unparseable response: ${line}` });
			}
		});
		spawned.on("error", () => {
			/* surfaced via exit below */
		});
		spawned.on("exit", () => {
			if (child === spawned) child = null;
			const waiting = pending;
			pending = [];
			for (const done of waiting) done({ error: "rex-tts exited" });
		});
		return spawned;
	}

	function send(req: Record<string, string>): Promise<Resp> {
		const c = engine();
		if (!c) return Promise.resolve({ error: `cannot start ${BIN}` });
		return new Promise((resolve) => {
			pending.push(resolve);
			c.stdin.write(`${JSON.stringify(req)}\n`);
		});
	}

	let last = "";

	// Load the model as soon as the session opens so the first answer
	// doesn't wait on model load.
	pi.on("session_start", async () => {
		engine();
	});

	pi.on("before_agent_start", async (event) => ({
		systemPrompt: event.systemPrompt + SPEECH_PROMPT,
	}));

	pi.on("message_end", async (event) => {
		if (event.message.role !== "assistant") return;
		const text = textOf(event.message.content).trim();
		if (text) last = text;
	});

	pi.on("agent_settled", async () => {
		const text = last;
		last = "";
		if (!text) return;
		const r = await send({ cmd: "speak", text });
		if (r.error) console.error(`pi-tts: ${r.error}`);
	});

	pi.registerCommand("voice", {
		description: "list TTS voices, or switch: /voice ryan",
		handler: async (args, ctx) => {
			const name = (args ?? "").trim();
			const r = await send(name ? { cmd: "voice", name } : { cmd: "voice" });
			if (r.error) {
				ctx.ui.notify(`pi-tts: ${r.error}`, "error");
			} else {
				ctx.ui.notify(r.text ?? "ok", "info");
			}
		},
	});

	pi.on("session_shutdown", async () => {
		child?.stdin.end();
		child = null;
	});
}
