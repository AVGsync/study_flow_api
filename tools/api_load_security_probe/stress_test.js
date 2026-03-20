import http from 'k6/http'
import { check, sleep } from 'k6'

export let options = {
	stages: [
		{ duration: '10s', target: 10000 },
		{ duration: '20s', target: 50000 },
		{ duration: '10s', target: 0 },
	],
}

// Функция для генерации строки только из букв и цифр
function generateAlphanumeric(length) {
	const charset = 'abcdefghijklmnopqrstuvwxyz0123456789'
	let res = ''
	while (length--) res += charset[(Math.random() * charset.length) | 0]
	return res
}

export default function () {
	const url = 'http://localhost:8082/api/user'

	// Формируем чистый alphanum логин
	const ID = `${__VU}${__ITER}${generateAlphanumeric(4)}`
	const payload = JSON.stringify({
		login: `u${ID}`,
		email: `e${ID}@test.com`,
	})

	const params = {
		headers: {
			'Content-Type': 'application/json',
			Authorization:
				'Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ0eXBlIjoiYWNjZXNzIiwic3ViIjoiYWRtaW4iLCJpZCI6ImU2ODZiMGYzLTdmYTktNDgyNC05MzlhLWY1MjAyMWJkZmM4NCIsInJvbGUiOiJST0xFX1BPUlRBTF9BRE1JTiIsImV4cCI6MTc3NDA1NDYwM30.efPOL1x3hcu7rVpFwFs8cCkTnTauIIt61e5yaT-8QYA',
		},
	}

	let res = http.patch(url, payload, params)

	check(res, {
		'is status 200': r => r.status === 200,
	})

	sleep(0.1)
}
