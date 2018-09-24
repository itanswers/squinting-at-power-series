package main
import "fmt"

func factorial(n int) int {
	if n == 0 {
		return 1
	}

	return n * factorial(n-1)
}

type rat struct {
	num int
	den int
}

func gcd(u, v int) int {
	if u < 0 {
		return gcd(-u, v)
	}
	if u == 0 {
		return v
	}
	return gcd(v%u, u)
}

func ratmk(i, j int) rat {
	return rat{i, j}
}

func print(r rat) {
	if r.den == 1 {
		fmt.Println(r.num)
	} else {
		fmt.Println(r.num, "/", r.den)
	}
}

func i2tor(i, j int) rat {
	d := gcd(i, j)

	if j > 0 {
		return rat{i / d, j / d}
	} else {
		return rat{-i / d, j / d}
	}
}

func itor(i int) rat {
	return i2tor(i, 1)
}

func ratadd(r, s rat) rat {
	d := gcd(r.den, s.den)
	return i2tor(r.num*(s.den/d)+s.num*(r.den/d), r.den*(s.den/d))
}

func ratsub(r, s rat) rat {
	d := gcd(r.den, s.den)
	return i2tor(r.num*(s.den/d)-s.num*(r.den/d), r.den*(s.den/d))
}

func mul(r, s rat) rat {
	return i2tor(r.num*s.num, r.den*s.den)
}

// -----------------------------------------------------------------
type ps chan rat
type ps2 [2]ps
type signal chan bool

func psmk() ps {
	result := make(ps)
	go func() {
		var n int
		for ; ; n++ {
			result <- i2tor(1, factorial(n))
		}
	}()

	return result
}

func add(p1, p2 ps) ps {
	result := make(ps)
	go func() {
		for {
			result <- ratadd(<-p1, <-p2)
		}
	}()

	return result
}

func deriv(F ps) ps {
	D := make(ps)
	go func() {
		<-F // drop the constant, because we're differentiating.
		n := 1
		for {
			f := <-F
			D <- rat{n * f.num, f.den}
			n++
		}
	}()

	return D
}

func mulc(c rat, p ps) ps {
	result := make(ps)
	go func() {
		for {
			val := <-p
			result <- i2tor(c.num*val.num, c.den*val.den)
		}
	}()
	return result
}

// This is actually a signale delay.
func mulx(n int, p ps) ps {
	zero := itor(0)
	Z := make(ps)
	go func() {
		for ; n > 0; n-- {
			Z <- zero
		}

		for {
			Z <- <-p
		}
	}()
	return Z
}

func integ(c rat, U ps) ps {
	Z := make(ps)
	go func() {
		Z <- c
		for i := 1; ; i++ {
			Z <- mul(i2tor(1, i), <-U)
		}
	}()

	return Z
}

func Ones() ps {
	res := make(ps)
	go func() {
		one := rat{1, 1}
		for {
			res <- one
		}
	}()
	return res
}

func copy(F, C ps) {
	for {
		C <- <-F
	}
}

func ps2mk(F, G ps) ps2 {
	return ps2{F, G}
}

func do_split(F, F0, F1 ps) {
	// This causes a quadratic growth of processes.
	f := <-F
	H := make(ps)
	select {
	// if F0 is available first, we send the received value f to it first, as a part of the rendezvous.
	case F0 <- f:
		// fmt.Println("F0 <- f")
		// Split the tail of F in a next process.
		go do_split(F, F0, H)
		// Supply the value to the other channel.
		F1 <- f
		// Copy all the incoming values to the other channel.
		copy(H, F1)
	case F1 <- f:
		// fmt.Println("F1 <- f")
		go do_split(F, F1, H)
		F0 <- f
		copy(H, F0)
	}
}

// Optimization 1: Don't leave the old processes hanging.
type split_opt1 struct{}

func (o split_opt1) split(F ps) ps2 {
	FF := ps2mk(make(ps), make(ps))
	go o.do_split(F, FF[0], FF[1])
	return FF
}

func split(F ps) ps2 {
	FF := ps2mk(make(ps), make(ps))
	go do_split(F, FF[0], FF[1])
	return FF
}

func (o split_opt1) do_split(F, F0, F1 ps) {
	f := <-F
	sig := make(signal)
	select {
	case F0 <- f:
		go o.do_split_new(F, F0, F1, sig)
		o.do_split_old(f, F1, sig)
	case F1 <- f:
		go o.do_split_new(F, F1, F0, sig)
		o.do_split_old(f, F0, sig)
	}
}

func (o split_opt1) do_split_old(f rat, F1 ps, release signal) {
	F1 <- f         // Serve the current value.
	release <- true // Announce that you are being terminated.
}

func (o split_opt1) do_split_mid(f rat, F1 ps, wait signal, release signal) {
	<-wait                         // When you don't have to wait anymore...
	o.do_split_old(f, F1, release) // serve the current value.
}

func (o split_opt1) do_split_new(F, F0, F1 ps, wait signal) {
	release := make(signal)
	select {
	case <-wait: // the queue is empty
		o.do_split(F, F0, F1)
	case f := <-F: // there are values to serve
		F0 <- f
		go o.do_split_new(F, F0, F1, release) // Prepare for serving next values.
		o.do_split_mid(f, F1, wait, release)  // Serve the current value when ready.
	}
}

func psmul(F, G ps, splitter func(ps) ps2) ps {
	P := make(ps)
	go func() {
		f := <-F
		g := <-G
		FF := splitter(F)
		GG := splitter(G)
		P <- mul(f, g)
		fG := mulc(f, GG[0])
		gF := mulc(g, FF[0])
		xFG := mulx(1, psmul(FF[1], GG[1], splitter))
		for {
			P <- ratadd(ratadd(<-fG, <-gF), <-xFG)
		}
	}()

	return P
}

func main() {
	count := 10
	ones := Ones()
	adder := add(psmk(), psmk())
	mc := mulc(rat{12, 7}, psmk())
	mx := mulx(5, psmk())
	integr := integ(rat{3, 10}, psmk())
	ff := split(psmk())
	for i := 0; i < count; i++ {
		fmt.Println("Adder")
		print(<-adder)
		fmt.Println("Mul")
		print(<-mc)
		fmt.Println("Ones")
		print(<-ones)
		fmt.Println("MulX")
		print(<-mx)
		fmt.Println("Integ")
		print(<-integr)
	}

	fmt.Println("Basic splitter")
	for i := 0; i < count; i++ {
		fmt.Print("f1: ")			
		print(<-ff[0])
		fmt.Print("f2: ")
		print(<-ff[1])
	}

	fmt.Println()
	fmt.Println("Optimized splitter")
	opt_splitter := split_opt1{}
	oo_opt1 := psmul(psmk(), psmk(), opt_splitter.split)
	for i := 0; i < count; i++ {
		print(<-oo_opt1)
	}
}

