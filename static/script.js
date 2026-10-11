const MAX_WIDTH = MAX_HEIGHT = 1000,
	WATERMARK_SRC = "/static/res/wm.png",
	WATERMARK_WIDTH = 297,
	WATERMARK_HEIGHT = 103,
	IG_TEMPLATE_SRC = "/static/res/ig.png",
	IG_MIN_OFFSET_X = 0,
	IG_MIN_OFFSET_Y = 342,
	IG_MAX_WIDTH = 1080,
	IG_MAX_HEIGHT = 988,

	WATERMARK = new Image(),
	IG_TEMPLATE = new Image(),
	fanarts = new Array(),
	months = new Set(),
	last_updated_link = document.getElementById("last-updated-link"),
	month_input = document.getElementById("month_input"),
	month_div = document.getElementById("month_div"),
	get_button = document.getElementById("get_button"),
	progress_div = document.getElementById("progress_div"),
	progress_bar = document.getElementById("progress_bar"),
	progress_label = document.getElementById("progress_label"),
	selectall_button = document.getElementById("selectall_button"),
	selectnone_button = document.getElementById("selectnone_button"),
	controls_div = document.getElementById("controls"),
	opacity_range = document.getElementById("opacity_range"),
	content_div = document.getElementById("content"),
	canvas_link = document.getElementById("canvas-download"),
	canvas_ig = document.getElementById("instagram-canvas"),
	fanart_template = document.getElementById("fanart-template").innerHTML,
	radians_factor = 90 * Math.PI / 180,
	GIF_EXTENSIONS = new Set(["gif"]),
	VIDEO_EXTENSIONS = new Set(["mp4", "m4v", "mov", "webm", "ogv", "mkv"]),
	VIDEO_FPS = 30,
	VIDEO_MIME = [
		"video/mp4;codecs=avc1.640028,mp4a.40.2",
		"video/mp4;codecs=avc1,mp4a",
		"video/mp4",
		"video/webm;codecs=vp9,opus",
		"video/webm;codecs=vp8,opus",
		"video/webm",
	].find(t => window.MediaRecorder && MediaRecorder.isTypeSupported(t)),
	VIDEO_EXT = VIDEO_MIME && VIDEO_MIME.startsWith("video/mp4") ? "mp4" : "webm";

let	new_entries = 0,
	audio_ctx = null;

WATERMARK.src = WATERMARK_SRC;
IG_TEMPLATE.src = IG_TEMPLATE_SRC;
month_input.addEventListener("keyup", (e) => { if (e.key === "Enter") getArtworks(); });

function addCanvasEvents(element) {
	const c = element.canvas;
	function place(e) {
		if (element.enabled == 0) return;
		if (e.button != 0) return;
		element.watermark.opacity = opacity_range.value;
		[element.watermark.x, element.watermark.y] = clickCoordsToCanvas(e.clientX, e.clientY, c);
		element.hovering = true;
		renderEntry(element);
	}
	if (element.clicked) {
		c.addEventListener('mousedown', place);
		return;
	}
	c.addEventListener('mousemove', place);
	c.addEventListener('mouseleave', () => {
		element.hovering = false;
		if (!element.clicked && element.enabled)
			renderEntry(element);
	});
	c.addEventListener('mousedown', (e) => {
		element.clicked = true;
		place(e);
		c.removeEventListener('mousemove', place);
		c.addEventListener('mousedown', place);
	});
}

function createElementFromHTML(htmlString) {
	const div = document.createElement('div');
	div.innerHTML = htmlString.trim();
	return div.firstChild;
}

function getMediaKind(path) {
	const ext = path.split(".").pop().toLowerCase();
	if (GIF_EXTENSIONS.has(ext)) return "gif";
	if (VIDEO_EXTENSIONS.has(ext)) return "video";
	return "image";
}

function isAnimated(element) {
	return element.media ? element.media.animated : element.kind != "image";
}

function getSaveFilename(element) {
	const ext = isAnimated(element) && VIDEO_MIME ? VIDEO_EXT : "png";
	return `${('0' + element.index).slice(-2)} - ${element.name}.${ext}`;
}

function loadImage(element) {
	return new Promise((resolve, reject) => {
		const img = new Image();
		img.addEventListener("load", () => resolve({ animated: false, source: img, width: img.width, height: img.height }));
		img.addEventListener("error", reject);
		img.src = element.content;
	});
}

function loadVideo(element) {
	return new Promise((resolve, reject) => {
		const video = document.createElement("video");
		video.muted = true;
		video.loop = true;
		video.playsInline = true;
		video.preload = "auto";
		video.addEventListener("loadeddata", () => {
			video.play().catch(() => {});
			resolve({
				animated: true,
				source: video,
				width: video.videoWidth,
				height: video.videoHeight,
				get duration() { return isFinite(video.duration) ? video.duration * 1000 : 10000; },
				restart: () => new Promise((res) => {
					video.addEventListener("seeked", res, { once: true });
					video.currentTime = 0;
					video.play().catch(() => {});
				}),
			});
		}, { once: true });
		video.addEventListener("error", reject, { once: true });
		video.src = element.content;
	});
}

async function loadGif(element) {
	if (!("ImageDecoder" in window) || !(await ImageDecoder.isTypeSupported("image/gif")))
		return loadImage(element); // fallback: first frame only

	const data = await (await fetch(element.content)).arrayBuffer();
	const decoder = new ImageDecoder({ data, type: "image/gif" });
	await decoder.tracks.ready;
	const count = decoder.tracks.selectedTrack.frameCount;
	if (count <= 1) {
		decoder.close();
		return loadImage(element);
	}

	const frames = [];
	let width, height, total = 0;
	for (let i = 0; i < count; i++) {
		const { image } = await decoder.decode({ frameIndex: i });
		if (width === undefined) {
			const f = Math.min(1, getFactor(image.displayWidth, image.displayHeight, MAX_WIDTH, MAX_HEIGHT));
			width = Math.ceil(image.displayWidth * f);
			height = Math.ceil(image.displayHeight * f);
		}
		const bitmap = await createImageBitmap(image, { resizeWidth: width, resizeHeight: height, resizeQuality: "high" });
		// browsers treat very short frame delays as 100ms
		let duration = (image.duration || 0) / 1000;
		if (duration <= 10) duration = 100;
		image.close();
		frames.push({ bitmap, end: total + duration });
		total += duration;
	}
	decoder.close();

	const media = {
		animated: true,
		width,
		height,
		duration: total,
		start: performance.now(),
		get source() {
			const t = (performance.now() - media.start) % total;
			return (frames.find(f => t < f.end) || frames[frames.length - 1]).bitmap;
		},
		restart: async () => { media.start = performance.now(); },
	};
	return media;
}

function loadMedia(element) {
	const fallback = (e) => {
		if (element.kind == "video") throw e;
		console.warn("Could not load as image, trying as video:", element.content);
		element.kind = "video";
		return loadVideo(element);
	};
	switch (element.kind) {
		case "video": return loadVideo(element);
		case "gif": return loadGif(element).catch(fallback);
		default: return loadImage(element).catch(fallback);
	}
}

function getNewCardHtml(element) {
	const id = element.id,
		index = element.index,
		name = element.name,
		content = element.content,
		text = `${element.name} → “${element.filename}”`,
		disabled = element.enabled == 0 ? " entry-disabled" : "",
		save_filename = getSaveFilename(element);
	const html_string = Mustache.render("{{={| |}=}}" + fanart_template, { id, index, name, content, text, disabled, save_filename });
	element.div = createElementFromHTML(html_string);
	const canvas = element.canvas = element.div.getElementsByTagName("canvas")[0];
	if (!element.mediaPromise) element.mediaPromise = loadMedia(element);
	element.mediaPromise.then((media) => {
		if (element.canvas !== canvas) return; // card was re-rendered in the meantime
		element.media = media;
		canvas.setAttribute("data-filename", getSaveFilename(element));
		renderEntry(element);
		addCanvasEvents(element);
	}).catch((e) => console.error("Could not load", element.content, e));
	return element.div;
}

async function updateFanartList() {
	content_div.innerHTML = "";
	get_button.disabled = false;
	get_button.innerText = emoji_get;

	let i = 0;
	for (fanart of fanarts) {
		if (fanart.enabled) {
			i++;
			fanart.index = i;
			content_div.appendChild(getNewCardHtml(fanart));
		}
	}
	for (fanart of fanarts) {
		if (!fanart.enabled) {
			fanart.index = 0;
			content_div.appendChild(getNewCardHtml(fanart));
		}
	}
}

function getFanart(id) {
	return fanarts.find(element => element.id == id);
}

function toggleEntry(id) {
	entry = getFanart(id);
	if (!entry) return;

	entry.enabled = !entry.enabled;
	entry.clicked = false;
	updateFanartList();
}

function saveCanvas(my_canvas, filename) {
	canvas_link.href = my_canvas.toDataURL("image/png").replace("image/png", "image/octet-stream");
	canvas_link.setAttribute("download", filename);
	canvas_link.click();
}

function saveBlob(blob, filename) {
	const url = URL.createObjectURL(blob);
	canvas_link.href = url;
	canvas_link.setAttribute("download", filename);
	canvas_link.click();
	setTimeout(() => URL.revokeObjectURL(url), 60000);
}

function reloadEntry(id) {
	const fanart = getFanart(id);
	if (!fanart) return;

	const old_div = fanart.div;
	content_div.replaceChild(getNewCardHtml(fanart), old_div);
	old_div.remove();
}

function selectAllNone(toggle) {
	fanarts.map(x => x.enabled = toggle)
	updateFanartList();
}

function clickCoordsToCanvas(clickX, clickY, c) {
	const rect = c.getBoundingClientRect();
	const x = (clickX - rect.left) * c.width / c.clientWidth;
	const y = (clickY - rect.top) * c.height / c.clientHeight;
	return [x, y];
}

function drawWatermark(wm_info, ctx) {
	ctx.globalAlpha = wm_info.opacity;
	ctx.filter = wm_info.invert;
	ctx.drawImage(WATERMARK, wm_info.x - (WATERMARK_WIDTH / 2), wm_info.y - (WATERMARK_HEIGHT / 2), WATERMARK_WIDTH, WATERMARK_HEIGHT);
}

function renderEntry(element, c = element.canvas, preview = true) {
	if (!element.media) return;
	setBaseImage(element, c);
	if (element.clicked || (preview && element.hovering))
		drawWatermark(element.watermark, c.getContext("2d"));
}

function animationLoop() {
	for (const fanart of fanarts)
		if (fanart.media && fanart.media.animated && fanart.div && fanart.div.isConnected)
			renderEntry(fanart);
	requestAnimationFrame(animationLoop);
}
requestAnimationFrame(animationLoop);

function updateOpacity() {
	opacity_label.innerHTML = Math.round(opacity_range.value * 100) + '%';
}

function getFactor(img_width, img_height, max_width, max_height) {
	return Math.min(max_width / img_width, max_height / img_height);
}

function setBaseImage(element, c = element.canvas) {
	const media = element.media;
	const ctx = c.getContext("2d");
	const f = getFactor(media.width, media.height, MAX_WIDTH, MAX_HEIGHT);

	const pre_width = Math.ceil(media.width * f);
	const pre_height = Math.ceil(media.height * f);

	if (element.rotated % 2) [c.width, c.height] = [pre_height, pre_width];
	else [c.width, c.height] = [pre_width, pre_height];
	ctx.imageSmoothingEnabled = (f < 1);

	const rotation_radians = radians_factor * element.rotated;
	ctx.rotate(rotation_radians);

	offset_x = (element.rotated == 2 || element.rotated == 3) ? -pre_width : 0;
	offset_y = (element.rotated == 1 || element.rotated == 2) ? -pre_height : 0;
	ctx.drawImage(media.source, offset_x, offset_y, pre_width, pre_height);
	ctx.rotate(-rotation_radians);
}

function rotateEntry(id) {
	const entry = fanarts.find(element => element.id == id);
	if (!entry) return;
	if (!entry.enabled) return;

	entry.rotated++;
	if (entry.rotated >= 4 || entry.rotated < 0) entry.rotated = 0;
	entry.clicked = false;
	updateFanartList();
}

function moveUpDown(id, amount) {
	const entry = fanarts.find(element => element.id == id);
	if (!entry) return;
	if (!entry.enabled) return;
	const pos = fanarts.indexOf(entry);
	const new_pos = pos + amount;

	if (new_pos <= -1 || new_pos >= fanarts.length) return;
	[fanarts[pos], fanarts[new_pos]] = [fanarts[new_pos], fanarts[pos]];
	updateFanartList();
}

function toggleInvert(id, button) {
	const entry = fanarts.find(element => element.id == id);
	if (!entry) return;
	if (!entry.enabled) return;
	entry.watermark.invert = entry.watermark.invert == '' ? 'invert(1)' : '';
	button.innerText = entry.watermark.invert ? emoji_color_black : emoji_color;
	renderEntry(entry);
}

async function postData(url = "", data = {}) {
	const response = await fetch(url, { method: "POST", headers: { "Content-Type": "application/x-www-form-urlencoded" }, body: new URLSearchParams(data).toString() });
	return response.json();
}

function getAudioTracks(media) {
	if (!(media.source instanceof HTMLVideoElement)) return [];
	try {
		if (!audio_ctx) audio_ctx = new AudioContext();
		if (!media.audio) {
			// route the audio only into the recording, so the preview stays silent
			const src = audio_ctx.createMediaElementSource(media.source);
			media.audio = audio_ctx.createMediaStreamDestination();
			src.connect(media.audio);
			media.source.muted = false;
		}
		audio_ctx.resume();
		return media.audio.stream.getAudioTracks();
	} catch (e) {
		console.warn("Could not capture audio:", e);
		return [];
	}
}

function composeIG(my_canvas, dest) {
	const f = getFactor(my_canvas.width, my_canvas.height, IG_MAX_WIDTH, IG_MAX_HEIGHT);
	const width = Math.ceil(my_canvas.width * f);
	const height = Math.ceil(my_canvas.height * f);
	const offset_x = Math.round((IG_MAX_WIDTH - width) / 2);
	const offset_y = Math.round((IG_MAX_HEIGHT - height) / 2);
	const destCtx = dest.getContext('2d');
	destCtx.drawImage(IG_TEMPLATE, 0, 0);
	destCtx.drawImage(my_canvas, IG_MIN_OFFSET_X + offset_x, IG_MIN_OFFSET_Y + offset_y, width, height);
}

async function recordEntry(entry, ig, button) {
	const filename = (ig ? "IG - " : "") + getSaveFilename(entry);
	const media = entry.media;
	const base = document.createElement("canvas");
	let target = base;
	if (ig) {
		target = document.createElement("canvas");
		[target.width, target.height] = [canvas_ig.width, canvas_ig.height];
	}
	const draw = () => {
		renderEntry(entry, base, false);
		if (ig) composeIG(base, target);
	};

	if (!VIDEO_MIME) {
		// MediaRecorder not supported: save the current frame
		draw();
		saveCanvas(target, filename.replace(/\.\w+$/, ".png"));
		return;
	}

	const old_text = button && button.innerText;
	if (button) { button.disabled = true; button.innerText = "⏳"; }

	try {
		await media.restart();
		draw();
		const stream = target.captureStream(VIDEO_FPS);
		getAudioTracks(media).forEach(t => stream.addTrack(t));
		const recorder = new MediaRecorder(stream, { mimeType: VIDEO_MIME, videoBitsPerSecond: 8000000 });
		const chunks = [];
		recorder.addEventListener("dataavailable", (e) => { if (e.data.size) chunks.push(e.data); });
		const stopped = new Promise(res => recorder.addEventListener("stop", res, { once: true }));

		let recording = true;
		const tick = () => { if (!recording) return; draw(); requestAnimationFrame(tick); };
		recorder.start();
		tick();
		await new Promise(res => setTimeout(res, media.duration));
		recording = false;
		recorder.stop();
		await stopped;
		saveBlob(new Blob(chunks, { type: VIDEO_MIME.split(";")[0] }), filename);
	} catch (e) {
		console.error("Could not record", entry.content, e);
		alert("Impossibile salvare il video: " + e);
	} finally {
		if (button) { button.disabled = false; button.innerText = old_text; }
	}
}

function saveFanart(entry, button) {
	if (isAnimated(entry)) return recordEntry(entry, false, button);
	saveCanvas(entry.canvas, entry.canvas.getAttribute("data-filename"));
}

function saveFanartIG(entry, button) {
	if (isAnimated(entry)) return recordEntry(entry, true, button);
	composeIG(entry.canvas, canvas_ig);
	saveCanvas(canvas_ig, "IG - " + entry.canvas.getAttribute("data-filename"));
}

function saveEntry(id, button) {
	const entry = getFanart(id);
	if (!entry || !entry.media) return;
	if (!entry.enabled) return;
	saveFanart(entry, button);
}

function saveEntryIG(id, button) {
	const entry = getFanart(id);
	if (!entry || !entry.media) return;
	if (!entry.enabled) return;
	saveFanartIG(entry, button);
}

function setProgress(done, total) {
	if (total == 0) {
		progress_div.hidden = true;
		return;
	}
	progress_div.hidden = false;
	progress_bar.style.width = Math.round(done / total * 100) + "%";
	progress_label.innerText = done < total
		? `Download in corso: ${done}/${total} (ne mancano ${total - done})`
		: `Download completato: ${done}/${total}`;
}

async function fetchArtworks(month_value) {
	const res = await fetch("/get?month=" + month_value);
	if (!res.ok) throw new Error(await res.text());

	// the server streams newline-delimited JSON: progress lines, then the entries
	const reader = res.body.getReader();
	const decoder = new TextDecoder();
	let buffer = "";
	while (true) {
		const { value, done } = await reader.read();
		if (value) buffer += decoder.decode(value, { stream: true });
		const lines = buffer.split("\n");
		buffer = lines.pop();
		for (const line of lines) {
			if (!line.trim()) continue;
			const msg = JSON.parse(line);
			if (msg.error) throw new Error(msg.error);
			if (msg.entries) return msg.entries;
			setProgress(msg.done, msg.total);
		}
		if (done) throw new Error("Risposta incompleta dal server.");
	}
}

async function getArtworks() {
	const month_value = month_input.value;
	if (months.has(month_value)) return;
	get_button.disabled = true;
	get_button.innerText = "🔄";
	setProgress(0, 0);
	try {
		const data = await fetchArtworks(month_value);
		fanarts.push(...data.map((element) => {
			return {
				...element,
				"kind": getMediaKind(element.content),
				"enabled": 1,
				"rotated": 0,
				"watermark": { invert: "" },
			};
		}));
		controls_div.hidden = false;
		updateOpacity();
		updateFanartList();
		months.add(month_value);
		setTimeout(() => setProgress(0, 0), 1500);
	} catch (e) {
		console.error(e);
		setProgress(0, 0);
		alert("Impossibile caricare le fanart: " + e.message);
		get_button.disabled = false;
		get_button.innerText = emoji_get;
	}
}

function saveAll() {
	const response = confirm("Vuoi davvero scaricare tutte le fanart?");
	if (response == false) return;
	fanarts.forEach((fanart) => {
		if (fanart.enabled && fanart.media)
			saveFanart(fanart);
	})
}

function saveAllIG() {
	const response = confirm("Vuoi davvero scaricare tutte le storie per le fanart?");
	if (response == false) return;
	fanarts.forEach((fanart) => {
		if (fanart.enabled && fanart.media)
			saveFanartIG(fanart);
	});
}

function updateDb() {
	postData("/update").then((data) => {
		last_updated_link.innerText = data.timestamp;
		if (data.new == 0) return;
		new_entries += data.new;
		last_updated_link.innerText += ` (+${ new_entries })`;
	});
}

function toClipBoard(id) {
	const fanart = getFanart(id);
	navigator.clipboard.writeText(fanart.canvas.getAttribute("data-filename"));
}
