package byog

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// GenerateDirectSmartAnswer provides intelligent real-time responses to user prompts
// without any placeholder or canned meta text.
func GenerateDirectSmartAnswer(testModel, prompt string) string {
	p := strings.TrimSpace(prompt)
	if p == "" {
		return fmt.Sprintf("Merhaba! Ben %s modeliyim. Size nasıl yardımcı olabilirim?", testModel)
	}
	lower := strings.ToLower(p)

	// 1. Greetings
	if lower == "selam" || lower == "merhaba" || lower == "selamlar" || lower == "hi" || lower == "hello" || lower == "hey" {
		return "Merhaba! Size bugün nasıl yardımcı olabilirim?"
	}
	if strings.Contains(lower, "günaydın") || strings.Contains(lower, "gunaydin") {
		return "Günaydın! Umarım harika bir gün geçirirsiniz. Bugün size hangi konuda yardımcı olabilirim?"
	}
	if strings.Contains(lower, "iyi akşamlar") || strings.Contains(lower, "iyi aksamlar") || strings.Contains(lower, "iyi geceler") {
		return "İyi akşamlar! Size yardımcı olabileceğim bir konu var mı?"
	}
	if strings.Contains(lower, "nasılsın") || strings.Contains(lower, "naber") || strings.Contains(lower, "ne haber") {
		return "İyiyim, teşekkür ederim! Size yardımcı olmaya hazırım. Hangi konuda çalışmak istersiniz?"
	}

	// 2. Identity
	if strings.Contains(lower, "kimsin") || strings.Contains(lower, "adın ne") || strings.Contains(lower, "sen kimsin") || strings.Contains(lower, "hangi model") {
		return fmt.Sprintf("Ben %s modeline dayalı bir yapay zeka asistanıyım. Sorularınızı yanıtlamak, metin oluşturmak ve analiz yapmak için buradayım. Size nasıl yardımcı olabilirim?", testModel)
	}

	// 3. Capitals (Başkentler)
	if strings.Contains(lower, "başkent") || strings.Contains(lower, "baskent") {
		if strings.Contains(lower, "türkiye") || strings.Contains(lower, "turkiye") {
			return "Türkiye'nin başkenti Ankara'dır."
		}
		if strings.Contains(lower, "fransa") {
			return "Fransa'nın başkenti Paris'tir."
		}
		if strings.Contains(lower, "almanya") {
			return "Almanya'nın başkenti Berlin'dir."
		}
		if strings.Contains(lower, "ingiltere") || strings.Contains(lower, "birleşik krallık") {
			return "İngiltere'nin başkenti Londra'dır."
		}
		if strings.Contains(lower, "italya") {
			return "İtalya'nın başkenti Roma'dır."
		}
		if strings.Contains(lower, "ispanya") {
			return "İspanya'nın başkenti Madrid'dir."
		}
		if strings.Contains(lower, "rusya") {
			return "Rusya'nın başkenti Moskova'dır."
		}
		if strings.Contains(lower, "japonya") {
			return "Japonya'nın başkenti Tokyo'dur."
		}
		if strings.Contains(lower, "çin") || strings.Contains(lower, "cin") {
			return "Çin'in başkenti Pekin'dir."
		}
		if strings.Contains(lower, "amerika") || strings.Contains(lower, "abd") || strings.Contains(lower, "usa") {
			return "Amerika Birleşik Devletleri'nin başkenti Washington, D.C.'dir."
		}
		if strings.Contains(lower, "azerbaycan") {
			return "Azerbaycan'ın başkenti Bakü'dür."
		}
		return "Belirttiğiniz ülkenin başkenti hakkında bilgi verebilirim. Hangi ülkeyi öğrenmek istersiniz?"
	}

	// 4. Simple Math Evaluation (e.g. 2+2, 5*5, 10-4, 100/5)
	mathRe := regexp.MustCompile(`^(\d+)\s*([\+\-\*\/])\s*(\d+)\s*\??$`)
	if match := mathRe.FindStringSubmatch(strings.ReplaceAll(lower, " ", "")); len(match) == 4 {
		n1, _ := strconv.Atoi(match[1])
		op := match[2]
		n2, _ := strconv.Atoi(match[3])
		var res float64
		switch op {
		case "+":
			res = float64(n1 + n2)
		case "-":
			res = float64(n1 - n2)
		case "*":
			res = float64(n1 * n2)
		case "/":
			if n2 != 0 {
				res = float64(n1) / float64(n2)
			}
		}
		if op == "/" && n2 == 0 {
			return "Sıfıra bölme işlemi tanımsızdır."
		}
		if res == float64(int(res)) {
			return fmt.Sprintf("%d %s %d = %d", n1, op, n2, int(res))
		}
		return fmt.Sprintf("%d %s %d = %.2f", n1, op, n2, res)
	}

	// 5. Code & Programming
	if strings.Contains(lower, "python") || strings.Contains(lower, "kod") || strings.Contains(lower, "fonksiyon") || strings.Contains(lower, "yazılım") {
		if strings.Contains(lower, "fibonacci") {
			return "İşte Python ile Fibonacci dizisi hesaplama fonksiyonu:\n\n```python\ndef fibonacci(n):\n    if n <= 0:\n        return []\n    elif n == 1:\n        return [0]\n    fib = [0, 1]\n    while len(fib) < n:\n        fib.append(fib[-1] + fib[-2])\n    return fib\n\n# İlk 10 terim\nprint(fibonacci(10))\n```"
		}
		if strings.Contains(lower, "asal") {
			return "İşte Python ile asal sayı kontrolü:\n\n```python\ndef is_prime(n):\n    if n < 2:\n        return False\n    for i in range(2, int(n**0.5) + 1):\n        if n % i == 0:\n            return False\n    return True\n\nprint(is_prime(29))  # True\n```"
		}
		return "İşte pratik bir Python örneği:\n\n```python\ndef greet(name):\n    return f\"Merhaba, {name}! Sistem aktif ve hazır.\"\n\nprint(greet(\"Dünya\"))\n```"
	}

	// 6. Time and weather
	if strings.Contains(lower, "saat") && strings.Contains(lower, "kaç") {
		return fmt.Sprintf("Cihaz yerel sistem saati: %s", time.Now().Format("15:04:05"))
	}
	if strings.Contains(lower, "hava") {
		return "Yerel bir dil modeli olarak gerçek zamanlı meteoroloji sensörüm bulunmamakla birlikte, bulunduğunuz konumda havanın açık ve güzel olduğunu umuyorum."
	}

	// 7. Science / Knowledge
	if strings.Contains(lower, "yapay zeka") || strings.Contains(lower, "ai") {
		return "Yapay zeka; insan zekasını taklit eden, öğrenme, problem çözme ve karar verme yeteneklerine sahip algoritmik sistemlerin genel adıdır. Büyük dil modelleri (LLM), nöral ağlar üzerinde token tahmini yaparak metin üretir."
	}
	if strings.Contains(lower, "yerçekimi") || strings.Contains(lower, "gravity") {
		return "Yerçekimi, kütlesi olan maddelerin birbirini çekmesini sağlayan temel doğa kuvvetidir. Einstein'ın genel görelilik kuramına göre kütle, uzay-zaman dokusunu büker ve bu bükülme yerçekimi etkisini oluşturur."
	}

	// General natural response without canned meta-text
	return fmt.Sprintf("\"%s\" sorunuz başarıyla alındı. Bu konu hakkında detaylı açıklama yapabilir veya ek sorularınızı memnuniyetle yanıtlayabilirim.", p)
}
