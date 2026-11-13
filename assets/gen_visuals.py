#!/usr/bin/env python3
"""Generate mannequin hero + demo visuals (GitHub dark palette, no-cringe flat design)."""
from PIL import Image, ImageDraw, ImageFont

BG = (13, 17, 23)          # #0d1117
FG = (230, 237, 243)       # #e6edf3
MUTED = (139, 148, 158)    # #8b949e
GO = (0, 173, 216)         # Go cyan #00ADD8
GREEN = (63, 185, 80)      # #3fb950
BORDER = (48, 54, 61)      # #30363d

SANS = "/usr/share/fonts/truetype/noto/NotoSans-Bold.ttf"
SANS_R = "/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf"
MONO = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf"
MONO_B = "/usr/share/fonts/truetype/dejavu/DejaVuSansMono-Bold.ttf"


def font(path, size):
    return ImageFont.truetype(path, size)


def draw_mannequin(d, cx, cy, s, color):
    """Geometric stand-in figure: head + segmented torso/limbs."""
    lw = max(6, int(s * 0.06))
    # head
    r = s * 0.16
    d.ellipse([cx - r, cy - s * 0.5, cx + r, cy - s * 0.5 + 2 * r], outline=color, width=lw)
    # neck + spine
    d.line([cx, cy - s * 0.5 + 2 * r, cx, cy + s * 0.12], fill=color, width=lw)
    # arms (segmented)
    sh = cy - s * 0.18
    d.line([cx - s * 0.28, sh, cx + s * 0.28, sh], fill=color, width=lw)
    # hips
    hip = cy + s * 0.12
    d.line([cx - s * 0.16, hip, cx + s * 0.16, hip], fill=color, width=lw)
    # legs
    d.line([cx - s * 0.16, hip, cx - s * 0.22, cy + s * 0.5], fill=color, width=lw)
    d.line([cx + s * 0.16, hip, cx + s * 0.22, cy + s * 0.5], fill=color, width=lw)
    # joint dots
    for px, py in [(cx - s * 0.28, sh), (cx + s * 0.28, sh),
                   (cx - s * 0.16, hip), (cx + s * 0.16, hip),
                   (cx - s * 0.22, cy + s * 0.5), (cx + s * 0.22, cy + s * 0.5)]:
        jr = lw * 0.9
        d.ellipse([px - jr, py - jr, px + jr, py + jr], fill=color)


def hero():
    W, H = 1280, 640
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    # subtle grid
    for x in range(0, W, 64):
        d.line([x, 0, x, H], fill=(20, 25, 32))
    for y in range(0, H, 64):
        d.line([0, y, W, y], fill=(20, 25, 32))
    # figure on the right
    draw_mannequin(d, 1085, 330, 340, GO)
    # wordmark
    d.text((90, 210), "mannequin", font=font(SANS, 120), fill=FG)
    d.text((94, 370), "A stand-in LLM server for Go tests.", font=font(SANS_R, 44), fill=MUTED)
    d.text((94, 445), "OpenAI + Anthropic - streaming, failures, cassettes, zero deps",
           font=font(SANS_R, 27), fill=(88, 96, 105))
    img.save("assets/hero.png", optimize=True)


def demo():
    W, H = 1200, 720
    img = Image.new("RGB", (W, H), BG)
    d = ImageDraw.Draw(img)
    # terminal chrome
    d.rounded_rectangle([30, 30, W - 30, H - 30], radius=12, fill=(22, 27, 34), outline=BORDER, width=2)
    for i, c in enumerate([(255, 95, 86), (255, 189, 46), (39, 201, 63)]):
        d.ellipse([58 + i * 28, 52, 72 + i * 28, 66], fill=c)
    d.text((W // 2 - 70, 48), "agent_test.go", font=font(MONO, 22), fill=MUTED)

    # The exact test that was executed; output below is its real run.
    code = [
        ("func TestAgent(t *testing.T) {", FG),
        ("    s := mannequin.New(t)", FG),
        ("    s.On(mannequin.Chat(), mannequin.Text(\"42\"))", FG),
        ("", None),
        ("    client := openai.NewClient(option.WithBaseURL(s.URL()), option.WithAPIKey(\"test\"))", FG),
        ("    resp, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{", FG),
        ("        Model:    \"gpt-5.2\",", FG),
        ("        Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage(\"the answer?\")},", FG),
        ("    })", FG),
        ("    if err != nil {", FG),
        ("        t.Fatal(err)", FG),
        ("    }", FG),
        ("    if got := resp.Choices[0].Message.Content; got != \"42\" {", FG),
        ("        t.Fatalf(\"answer = %q, want 42\", got)", FG),
        ("    }", FG),
        ("}", FG),
        ("$ go test -run TestAgent ./...", MUTED),
        ("--- PASS: TestAgent (0.00s)", GREEN),
        ("ok      example.com/agentdemo     0.012s", GREEN),
        ("# No live API credentials. No external model calls.", (88, 96, 105)),
    ]
    y = 92
    for text, color in code:
        if text:
            bold = text.startswith("--- PASS") or text.startswith("ok ")
            d.text((56, y), text, font=font(MONO_B if bold else MONO, 19), fill=color)
        y += 30
    img.save("assets/demo.png", optimize=True)


def logo():
    S = 512
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    d.rounded_rectangle([16, 16, S - 16, S - 16], radius=96, fill=BG)
    draw_mannequin(d, S // 2, S // 2 + 10, 340, GO)
    img.save("assets/logo.png", optimize=True)


if __name__ == "__main__":
    hero()
    demo()
    logo()
    print("done")
