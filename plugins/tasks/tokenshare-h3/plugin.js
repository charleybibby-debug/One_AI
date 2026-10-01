const MODEL = "tokenshare-minimax-h3";
const UPSTREAM_MODEL = "minimax-h3";
const MIN_DURATION = 4;
const MAX_DURATION = 15;
const DEFAULT_DURATION = 5;
const MAX_CONDITIONS = 2;
const MAX_INPUT_VIDEO_SECONDS = 15;
const ASPECT_RATIOS = ["16:9", "9:16", "1:1"];

const VIDEO_SECONDS = {
  type: "number",
  unit: "second",
  description: { en: "Video generation unit price", zh: "视频生成单价" },
};

export const meta = {
  apiVersion: 1,
  key: "tokenshare-h3",
  name: "TokenShare MiniMax H3",
  icon: "MiniMax.Color",
  description: {
    en: "TokenShare MiniMax-H3 asynchronous video generation",
    zh: "TokenShare MiniMax-H3 异步视频生成",
  },
  version: "1.0.0",
  author: { name: "QuantumNous" },
  baseUrl: "https://api.tokenshare.net",
  models: [MODEL],
  fetchMode: "per_task",
  usageSchema: {
    seconds: VIDEO_SECONDS,
    input_images: {
      type: "number",
      unit: "count",
      description: { en: "Input image unit price", zh: "输入图片单价" },
    },
    input_video_seconds: {
      type: "number",
      unit: "second",
      description: { en: "Input video unit price", zh: "输入视频单价" },
    },
  },
  usageExamples: [
    { label: "Text to video · 5s", facts: { seconds: 5, input_images: 0, input_video_seconds: 0 } },
    { label: "Keyframes · 5s", facts: { seconds: 5, input_images: 2, input_video_seconds: 0 } },
    { label: "Reference video · 5s", facts: { seconds: 5, input_images: 0, input_video_seconds: 15 } },
  ],
  protocols: [{ name: "openai_responses", supports: ["stream", "sync", "background"] }, "openai_video"],
};

function trimmed(value) {
  return String(value || "").trim();
}

function apiBase(value) {
  return trimmed(value).replace(/\/+$/, "").replace(/\/v1$/, "");
}

function durationFor(value) {
  if (value === undefined || value === null || value === "") return DEFAULT_DURATION;
  const seconds = Number(value);
  if (!Number.isInteger(seconds) || seconds < MIN_DURATION || seconds > MAX_DURATION) {
    throw new Error("minimax-h3 duration must be an integer between " + MIN_DURATION + " and " + MAX_DURATION + " seconds");
  }
  return seconds;
}

function aspectRatioFor(value) {
  const raw = trimmed(value);
  if (!raw) return "16:9";
  if (ASPECT_RATIOS.includes(raw)) return raw;
  const parts = raw.toLowerCase().replace("x", ":").split(":");
  if (parts.length === 2 && Number(parts[0]) > 0 && Number(parts[1]) > 0) {
    const width = Number(parts[0]);
    const height = Number(parts[1]);
    if (Math.abs(width / height - 1) < 0.01) return "1:1";
    return width > height ? "16:9" : "9:16";
  }
  throw new Error("minimax-h3 aspect ratio must be 16:9, 9:16, or 1:1");
}

function conditionsFor(req) {
  const metadata = req.metadata && typeof req.metadata === "object" ? req.metadata : {};
  const source = req.conditions !== undefined ? req.conditions : metadata.conditions;
  if (source === undefined || source === null) return [];
  if (!Array.isArray(source)) throw new Error("conditions must be an array");
  if (source.length > MAX_CONDITIONS) throw new Error("minimax-h3 accepts at most " + MAX_CONDITIONS + " conditions");
  return source.map(function (condition) {
    if (!condition || typeof condition !== "object" || Array.isArray(condition)) throw new Error("each condition must be an object");
    return condition;
  });
}

function conditionKind(condition) {
  const value = trimmed(condition.type || condition.kind || condition.role).toLowerCase();
  if (value.includes("video") || value === "video_audio") return "video";
  if (value.includes("image") || value === "keyframe" || value === "reference") return "image";
  return "other";
}

function taskFor(req, conditions) {
  const explicit = trimmed(req.task || (req.metadata && req.metadata.task)).toLowerCase();
  if (explicit === "t2va" || explicit === "fl2va" || explicit === "ref2va") return explicit;
  if (!conditions.length) return "t2va";
  return conditions.some(function (condition) {
    return conditionKind(condition) === "video" || trimmed(condition.role).toLowerCase() === "reference";
  })
    ? "ref2va"
    : "fl2va";
}

function durationFromRequest(req) {
  const target = req.target && typeof req.target === "object" ? req.target : {};
  if (target.duration_seconds !== undefined) return durationFor(target.duration_seconds);
  if (req.duration !== undefined) return durationFor(req.duration);
  return durationFor(req.seconds);
}

function targetFor(req) {
  const metadata = req.metadata && typeof req.metadata === "object" ? req.metadata : {};
  const inputTarget = req.target && typeof req.target === "object" ? req.target : {};
  const shortEdge = Number(inputTarget.short_edge === undefined ? metadata.short_edge || 768 : inputTarget.short_edge);
  if (shortEdge !== 768) throw new Error("minimax-h3 currently supports short_edge 768 only");
  const aspect = inputTarget.aspect_ratio || metadata.aspect_ratio || metadata.ratio || req.aspect_ratio || req.ratio || req.size;
  return { short_edge: 768, aspect_ratio: aspectRatioFor(aspect), duration_seconds: durationFromRequest(req) };
}

function promptFor(req) {
  const prompt = trimmed(req.prompt || req.input);
  if (!prompt) throw new Error("prompt is required");
  return prompt;
}

function requestParts(req) {
  const conditions = conditionsFor(req);
  const body = { model: UPSTREAM_MODEL, task: taskFor(req, conditions), prompt: promptFor(req), target: targetFor(req) };
  if (conditions.length) body.conditions = conditions;
  return { body: body, conditions: conditions };
}

function imageConditions(images) {
  return images.map(function (url, index) {
    return { type: "image", role: "keyframe", frame_index: index === 0 ? 0 : -1, url: url };
  });
}

function videoText(ctx) {
  const artifact = ctx && ctx.artifacts && ctx.artifacts.video;
  const url = trimmed(artifact && artifact.url);
  if (!url) throw new Error("video artifact is unavailable");
  const escaped = url.replace(/&/g, "&amp;").replace(/\"/g, "&quot;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  return '<video controls src="' + escaped + '"></video>';
}

export function buildSubmitRequest(ctx) {
  const req = ctx.requestBody || {};
  const built = requestParts(req);
  const headers = { Authorization: "Bearer " + ctx.apiKey, "Content-Type": "application/json", Accept: "application/json" };
  const publicTaskId = trimmed(ctx.publicTaskId);
  if (publicTaskId) headers["Idempotency-Key"] = "new-api-" + publicTaskId;
  return { url: apiBase(ctx.baseUrl) + "/v1/videos", method: "POST", headers: headers, body: built.body, action: built.body.task === "t2va" ? "text_to_video" : "image_to_video" };
}

export function parseSubmitResponse(_ctx, resp) {
  const body = resp.body || {};
  const error = body.error && typeof body.error === "object" ? body.error : null;
  if (error && trimmed(error.message)) throw new Error(trimmed(error.message));
  const taskId = body.id || body.task_id;
  if (!taskId) throw new Error("TokenShare response did not include a video id");
  return { taskId: taskId, taskData: body };
}

function taskStatus(body) {
  const source = body && typeof body === "object" && !Array.isArray(body) ? body : {};
  const task = source.task && typeof source.task === "object" && !Array.isArray(source.task) ? source.task : source;
  return { task: task, status: trimmed(task.status || source.status).toLowerCase() };
}

export function parseTaskResult(_ctx, body, response) {
  const error = body && body.error && typeof body.error === "object" ? body.error : null;
  const responseStatus = Number(response && response.status);
  if (error && trimmed(error.message)) {
    if (responseStatus === 429 || responseStatus >= 500) throw new Error(trimmed(error.message));
    return { status: "FAILURE", progress: "100%", reason: trimmed(error.message) };
  }
  const parsed = taskStatus(body);
  const statuses = { queued: "QUEUED", running: "IN_PROGRESS", processing: "IN_PROGRESS", cancel_requested: "IN_PROGRESS", completed: "SUCCESS", canceled: "FAILURE", cancelled: "FAILURE", failed: "FAILURE", expired: "FAILURE" };
  const mapped = statuses[parsed.status];
  const result = { status: mapped || "UNKNOWN" };
  if (!mapped) {
    result.reason = "unrecognized status: " + parsed.status;
    return result;
  }
  if (mapped === "IN_PROGRESS") result.progress = parsed.status === "processing" ? "75%" : parsed.status === "cancel_requested" ? "80%" : parsed.status === "running" ? "50%" : "25%";
  if (mapped === "FAILURE") result.reason = trimmed(parsed.task.error && (parsed.task.error.message || parsed.task.error.detail)) || "video task failed";
  return result;
}

export function listArtifacts(task) {
  return task.status === "SUCCESS" ? [{ key: "video", type: "video", mimeType: "video/mp4" }] : [];
}

export function buildQueryRequest(ctx) {
  return { url: apiBase(ctx.baseUrl) + "/v1/videos/" + encodeURIComponent(ctx.taskId), method: "GET", headers: { Authorization: "Bearer " + ctx.apiKey, Accept: "application/json" } };
}

export function buildContentRequest(ctx) {
  if (ctx.artifactKey !== "video") throw new Error("artifact_not_found");
  return { url: apiBase(ctx.baseUrl) + "/v1/videos/" + encodeURIComponent(ctx.upstreamTaskId) + "/content", method: ctx.clientRequest.method, headers: { Authorization: "Bearer " + ctx.apiKey } };
}

function usageNumber(value) {
  const number = Number(value);
  return Number.isFinite(number) ? number : 0;
}

export function extractUsage(ctx) {
  const req = ctx.requestBody || {};
  const built = requestParts(req);
  const inputImages = built.conditions.filter(function (condition) {
    return conditionKind(condition) === "image";
  }).length;
  const hasVideo = built.conditions.some(function (condition) {
    return conditionKind(condition) === "video";
  });
  return { seconds: built.body.target.duration_seconds, input_images: inputImages, input_video_seconds: hasVideo ? MAX_INPUT_VIDEO_SECONDS : 0 };
}

export function extractUsageOnComplete(_task, _taskResult, body) {
  const parsed = taskStatus(body);
  const task = parsed.task || {};
  const usage = task.usage && typeof task.usage === "object" ? task.usage : {};
  const target = task.target && typeof task.target === "object" ? task.target : {};
  const facts = {};
  const seconds = usage.output_seconds === undefined ? target.duration_seconds : usage.output_seconds;
  const outputSeconds = usageNumber(seconds);
  if (outputSeconds >= MIN_DURATION && outputSeconds <= MAX_DURATION) facts.seconds = outputSeconds;
  const images = usage.input_image_count === undefined ? usage.input_images : usage.input_image_count;
  const imageCount = usageNumber(images);
  if (imageCount >= 0 && imageCount <= MAX_CONDITIONS && Number.isInteger(imageCount)) facts.input_images = imageCount;
  const inputSeconds = usage.input_video_seconds === undefined ? usage.input_seconds : usage.input_video_seconds;
  const videoSeconds = usageNumber(inputSeconds);
  if (videoSeconds >= 0 && videoSeconds <= MAX_INPUT_VIDEO_SECONDS) facts.input_video_seconds = videoSeconds;
  return Object.keys(facts).length ? facts : null;
}

function inputParts(req) {
  const texts = [];
  const images = [];
  const input = req.input;
  if (typeof input === "string") texts.push(input);
  else if (Array.isArray(input)) {
    for (const item of input) {
      if (typeof item === "string") {
        texts.push(item);
        continue;
      }
      if (!item || typeof item !== "object" || Array.isArray(item)) continue;
      const content = item.content === undefined ? [item] : Array.isArray(item.content) ? item.content : [item.content];
      for (const part of content) {
        if (typeof part === "string") texts.push(part);
        else if (part && typeof part === "object" && ["input_text", "text"].includes(part.type) && typeof part.text === "string") texts.push(part.text);
        else if (part && typeof part === "object" && ["input_image", "image_url"].includes(part.type)) {
          let image = part.image_url;
          if (image && typeof image === "object") image = image.url;
          if (trimmed(image)) images.push(trimmed(image));
        }
      }
    }
  }
  return { prompt: texts.filter(function (text) { return trimmed(text); }).join("\n"), images: images };
}

export const protocols = {
  openai_responses: {
    decodeRequest: function (ctx) {
      if (!ctx.body || ctx.body.kind !== "json") throw new Error("JSON body required");
      const req = ctx.body.value;
      if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
      const input = inputParts(req);
      const prompt = input.prompt || trimmed(req.prompt);
      if (!prompt) throw new Error("input is required");
      const clientModel = trimmed(ctx.model) || MODEL;
      const requestBody = Object.assign({}, req, { model: clientModel, prompt: prompt });
      if (input.images.length) requestBody.conditions = imageConditions(input.images);
      return { kind: "submit", model: clientModel, action: input.images.length ? "image_to_video" : "text_to_video", requestBody: requestBody };
    },
    renderEvents: function (ctx, task, previousState) {
      const status = String(task.status || "UNKNOWN").toUpperCase();
      const value = Number(String(task.progress || "").replace("%", ""));
      const progress = Number.isFinite(value) && value >= 0 && value <= 100 ? value : null;
      const state = { status: status, progress: progress };
      if (status === "SUCCESS") return { events: previousState && previousState.status === status ? [] : [{ type: "output", data: videoText(ctx) }], state: state, done: true };
      if (status === "FAILURE") return { events: [{ type: "error", code: "task_failed", message: task.fail_reason || "video task failed" }], state: state, done: true };
      if (previousState && previousState.status === status && previousState.progress === progress) return { events: [], state: state, done: false };
      const event = { type: "progress", message: status.toLowerCase() };
      if (progress !== null) event.progress = progress;
      return { events: [event], state: state, done: false };
    },
    renderFinal: function (ctx) {
      return { output: [{ type: "message", status: "completed", role: "assistant", content: [{ type: "output_text", text: videoText(ctx), annotations: [], logprobs: [] }] }], metadata: { vendor: "tokenshare" } };
    },
  },
  openai_video: {
    decodeRequest: function (ctx) {
      if (!ctx.body || (ctx.body.kind !== "json" && ctx.body.kind !== "multipart")) throw new Error("JSON or multipart body required");
      if (ctx.body.kind === "multipart" && (ctx.body.files || []).length) throw new Error("TokenShare H3 requires uploaded condition URLs; multipart files are not supported");
      const req = ctx.body.kind === "json" ? ctx.body.value : ctx.body.fields || {};
      if (!req || typeof req !== "object" || Array.isArray(req)) throw new Error("request body must be an object");
      const requestBody = Object.assign({}, req, { model: trimmed(ctx.model) || MODEL });
      if (typeof requestBody.metadata === "string") {
        try {
          requestBody.metadata = JSON.parse(requestBody.metadata);
        } catch (_error) {
          throw new Error("metadata must be a JSON object string");
        }
      }
      if (requestBody.seconds !== undefined && requestBody.duration === undefined) requestBody.duration = requestBody.seconds;
      return { kind: "submit", model: requestBody.model, action: "text_to_video", requestBody: requestBody };
    },
    render: function (_ctx, task) {
      const statuses = { NOT_START: "queued", SUBMITTED: "queued", QUEUED: "queued", IN_PROGRESS: "in_progress", SUCCESS: "completed", FAILURE: "failed" };
      return { id: task.task_id, object: "video", model: MODEL, status: statuses[task.status] || "unknown", progress: Number(String(task.progress || "0").replace("%", "")), created_at: Number(task.created_at || 0) };
    },
  },
};
