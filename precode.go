package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// Generator генерирует последовательность чисел и отправляет их в канал ch.
// Для каждого числа вызывается функция fn().
func Generator(ctx context.Context, ch chan int64, fn func(int64)) {
	defer close(ch) // Закрываем канал ch перед завершением функции

	n := int64(1) // Начальное значение
	for {
		select {
		case <-ctx.Done():
			// Если поступил сигнал об отмене контекста, завершаем функцию
			return
		case ch <- n:
			// Отправляем число в канал и вызываем fn
			fn(n)
			n++
		}
	}
}

// Worker читает числа из канала in и записывает их в канал out.
func Worker(in, out chan int64) {
	defer close(out) // Закрываем канал out перед завершением функции

	for {
		// Читаем число из канала in
		v, ok := <-in
		if !ok {
			// Если канал in закрыт, завершаем работу
			return
		}

		// Отправляем число в канал out
		out <- v

		// Делаем паузу на 1 миллисекунду
		time.Sleep(1 * time.Millisecond)
	}
}

func main() {
	chIn := make(chan int64)

	// Создаём контекст с таймаутом на 1 секунду
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel() // Освобождаем ресурсы при завершении

	// Переменные для подсчёта суммы и количества сгенерированных чисел
	var inputSum int64   // Сумма
	var inputCount int64 // Количество

	// Запускаем генератор чисел
	go Generator(ctx, chIn, func(i int64) {
		// Потокобезопасное увеличение переменных
		atomic.AddInt64(&inputSum, i)
		atomic.AddInt64(&inputCount, 1)
	})

	const NumOut = 10 // Количество обрабатывающих горутин
	// Слайс каналов для передачи данных от генератора к воркерам
	outs := make([]chan int64, NumOut)
	for i := 0; i < NumOut; i++ {
		outs[i] = make(chan int64)
		go Worker(chIn, outs[i])
	}

	// Слайс для подсчёта количества чисел, обработанных каждой горутиной
	amounts := make([]int64, NumOut)
	// Результирующий канал для сбора всех данных
	chOut := make(chan int64, NumOut)

	var wg sync.WaitGroup

	// Собираем числа из каналов outs
	for i, out := range outs {
		wg.Add(1) // Увеличиваем счётчик WaitGroup перед запуском горутины

		go func(i int, out <-chan int64) {
			defer wg.Done() // Уменьшаем счётчик при завершении горутины

			for val := range out {
				amounts[i]++ // Увеличиваем счётчик обработанных чисел
				chOut <- val // Отправляем число в результирующий канал
			}
		}(i, out)
	}

	// Горутина для закрытия chOut после завершения всех горутин
	go func() {
		wg.Wait()    // Ожидаем завершения всех горутин
		close(chOut) // Закрываем результирующий канал
	}()

	// Подсчёт количества и суммы чисел из результирующего канала
	var count int64 // Количество
	var sum int64   // Сумма

	for val := range chOut {
		count++    // Увеличиваем количество
		sum += val // Суммируем значение
	}

	// Вывод результатов
	fmt.Println("Количество чисел:", inputCount, count)
	fmt.Println("Сумма чисел:", inputSum, sum)
	fmt.Println("Разбивка по каналам:", amounts)

	// Проверка результатов
	if inputSum != sum {
		log.Fatalf("Ошибка: суммы чисел не равны: %d != %d\n", inputSum, sum)
	}
	if inputCount != count {
		log.Fatalf("Ошибка: количество чисел не равно: %d != %d\n", inputCount, count)
	}
	for _, v := range amounts {
		inputCount -= v
	}
	if inputCount != 0 {
		log.Fatalf("Ошибка: разделение чисел по каналам неверное\n")
	}
}
