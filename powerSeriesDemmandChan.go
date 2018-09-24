package main
import "fmt"

type demandChannel struct {
	request signal
	data    chan rat
}

type dcPair [2]demandChannel

func (F demandChannel) get() rat {
	F.request <- true
	return <-(F.data)
}

func (F demandChannel) put(d rat) {
	<-F.request
	F.data <- d
}

func mkDc() demandChannel {
	return demandChannel{
		make(signal),
		make(chan rat),
	}
}

func add(F, G demandChannel) demandChannel {
	result := mkDc()
	go func() {
		for {
			result.request <- true
			result.data <- ratadd(F.get(), G.get())
		}
	}()

	return result
}

func psmul(F, G demandChannel, splitter func(demandChannel) dcPair) demandChannel {
	P := mkDc()
	go func() {
		f := F.get()		
		g := G.get()
		FF := splitter(F)
		GG := splitter(G)
		P.put(mul(f, g))
		fG := mulc(f, GG[0])
		gF := mulc(g, FF[0])
		xFG := mulx(1, psmul(FF[1], GG[1], splitter))
		for {
			P.put(ratadd(ratadd(fG.get(), gF.get()), xFG.get()))
		}
	}()

	return P
}

func mulc(c rat, p demandChannel) demandChannel {
	result := mkDc()
	go func() {
		for {
			val := p.get()
			result.put(i2tor(c.num*val.num, c.den*val.den))
		}
	}()
	return result
}

// This is actually a signale delay.
func mulx(n int, p demandChannel) demandChannel {
	zero := itor(0)
	Z := mkDc()
	go func() {
		for ; n > 0; n-- {
			Z.put(zero)
		}

		for {
			Z.put(p.get())
		}
	}()
	return Z
}

type split_opt1 struct{}

func (o split_opt1) split(F demandChannel) dcPair {
	FF := dcPair{mkDc(), mkDc()}
	go o.do_split(F, FF[0], FF[1])
	return FF
}
func (o split_opt1) do_split(F, F0, F1 demandChannel) {
	f := F.get()
	sig := make(signal)
	select {
	case <-F0.request:
		F0.data <- f
		go o.do_split_new(F, F0, F1, sig)
		o.do_split_old(f, F1, sig)
	case <-F1.request:
		F1.data <- f
		go o.do_split_new(F, F1, F0, sig)
		o.do_split_old(f, F0, sig)
	}
}

func (o split_opt1) do_split_old(f rat, F1 demandChannel, release signal) {
	F1.put(f)       // Serve the current value.
	release <- true // Announce that you are being terminated.
}

func (o split_opt1) do_split_mid(f rat, F1 demandChannel, wait signal, release signal) {
	<-wait                         // When you don't have to wait anymore...
	o.do_split_old(f, F1, release) // serve the current value.
}

func (o split_opt1) do_split_new(F, F0, F1 demandChannel, wait signal) {
	release := make(signal)
	select {
	case <-wait: // the queue is empty
		o.do_split(F, F0, F1)
	case F.request <- true: // there are values to serve
		f := <-F.data
		F0.put(f)
		go o.do_split_new(F, F0, F1, release) // Prepare for serving next values.
		o.do_split_mid(f, F1, wait, release)  // Serve the current value when ready.
	}
}

func psDc() demandChannel {
	result := mkDc()
	go func() {
		var n int
		for ; ; n++ {
			result.put(i2tor(1, factorial(n)))
		}
	}()

	return result
}

func main() {
	count := 10
	ones1 := Ones()
	ones2 := Ones()
	opt_splitter := split_opt1{}
	fmt.Println("Multiply One by One: ")
	oo_opt1 := psmul(ones2, ones1, opt_splitter.split)
	for i := 0; i < count; i++ {
		select {
		case oo_opt1.request <- true:
			print(<-oo_opt1.data)
		}
	}
}

// -----------------------------------------------
// The following are copied from the base example.
// -----------------------------------------------

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

type signal chan bool

func print(r rat) {
	if r.den == 1 {
		fmt.Println(r.num)
	} else {
		fmt.Println(r.num, "/", r.den)
	}
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

func Ones() demandChannel {
	res := mkDc()
	go func() {
		one := rat{1, 1}
		for {
			res.put(one)
		}
	}()

	return res
}
