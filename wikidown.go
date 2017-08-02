package wikidown

import(
	"fmt"
	"image/jpeg"
	"image/png"
	"os"
	"regexp"
	"strconv"
	"strings"
	"html/template"

	"github.com/unixpickle/resize"
)

func ParseAll(s string) (string, [][]string) {
	var toc [][]string
	s = ParseNoEscape(s)
	s = template.HTMLEscapeString(s)
	s, toc = Header(s)
	s = Parse(s)
	return s, toc
}

func ParseNoEscape(s string) string {
	// replace lines starting with asterisk with percent sign (used for internal parsing reasons)
	var re = regexp.MustCompile(`(?m)^\*(.*)`)
	s = re.ReplaceAllString(s, `%$1`)

	// ''''' . ''''' to *** . *** (wiki -> markdown)
	re = regexp.MustCompile(`\'\'\'\'\'(.*?)\'\'\'\'\'`)
	s = re.ReplaceAllString(s, `***$1***`)

	// ''' . ''' to ** . ** (wiki -> markdown)
	re = regexp.MustCompile(`\'\'\'(.*?)\'\'\'`)
	s = re.ReplaceAllString(s, `**$1**`)

	// '' . '' to * . * (wiki -> markdown)
	re = regexp.MustCompile(`\'\'(.*?)\'\'`)
	s = re.ReplaceAllString(s, `*$1*`)

	// replace blockquote tag block with markdown syntax
	re = regexp.MustCompile(`<blockquote>(?:\n|\r\n)?((?:.)*)(?:\n|\r\n)?<\/blockquote>`)
	s = re.ReplaceAllString(s, `> $1`)

	// ^> . $ to ^\$ . $ (used for internal parsing reasons)
	re = regexp.MustCompile(`(?m)^> (.*)`)
	s = re.ReplaceAllString(s, `$ $1`)

	// remove any HTML-style comments
	re = regexp.MustCompile(`<!--.*?-->`)
	s = re.ReplaceAllString(s, ``)

	return s
}

func Parse(s string) string {
	// internal wiki image
	// [[File:Filename.jpg|thumb|Caption]]
	var re = regexp.MustCompile(`\[\[File:(.*?)\|(thumb(?:nail)?)\|(.*?)\]\]`)
	s = re.ReplaceAllString(s, `<img src="/img/$1" alt="$3">`)

	var reUnorderedList = regexp.MustCompile(`(?m)^%(.*)`)
	var reOrderedList = regexp.MustCompile(`(?m)^#(.*)`)
	var reDescriptionList = regexp.MustCompile(`(?m)^;(.*)`)
	var reSubDescriptionList = regexp.MustCompile(`(?m)^:(.*)`)
	var reImgSrc = regexp.MustCompile(`src="\/img\/(.*?)"`)

	// remove carriage returns from string
	s = strings.Replace(s, "\r", "" , -1)

	// split input string into an array of lines
	wordArr := strings.Split(s,"\n")

	countUnOrdList := 0
	match := false

	countOrdList := 0
	matchOrd := false

	countDList := 0
	matchList := false

	for i, v := range wordArr {
		// if we have an image, make sure there is an extant thumbnail
		if(reImgSrc.MatchString(v)) {
			first := v[0:strings.Index(v, "src")]
			second := reImgSrc.FindString(v)
			second =  strings.Replace(second, " ", "_", -1)
			third := v[strings.Index(v, "alt"):]
			imgFileName := reImgSrc.ReplaceAllString(second, `$1`)

			if _, err := os.Stat("img/"+imgFileName); !os.IsNotExist(err) {
				file, err := os.Open("img/"+imgFileName)
				if err != nil {
					fmt.Println("file does not exist")
				}
				// decode jpeg into image.Image
				img, err := jpeg.Decode(file)
				if err != nil {
					fmt.Println("file does not exist")
				}
				file.Close()

				g := img.Bounds().Size()
				srcW := g.X
				srcH := g.Y

				_, err = os.Stat("img/220px-"+imgFileName[0:strings.Index(imgFileName, ".")]+".jpg");
				if (srcW > 220) && os.IsNotExist(err) {
					resized := resize.Resize(220, 0, img, 2)

					out, err := os.Create("img/220px-"+imgFileName[0:strings.Index(imgFileName, ".")]+".jpg")
					if err != nil {
						fmt.Println(err)
					}
					defer out.Close()

					// unfortunately, image/jpeg does not support 4:4:4 chroma subsampling, so even a jpeg of quality of 100 will have washed out colors
					//jpeg.Encode(out, resized, &jpeg.Options{Quality: 100})

					png.Encode(out, resized)
				}
				_, err = os.Stat("img/220px-"+imgFileName[0:strings.Index(imgFileName, ".")]+".jpg");
				if !os.IsNotExist(err) {
					second = strings.Replace(second, `src="/img/`, `src="/img/220px-`, -1)
					second = second + ` data-file-width="` + strconv.Itoa(srcW) + `" data-file-height="` + strconv.Itoa(srcH) + `" `
				}
			}
			figCaption := strings.Replace(imgFileName[0:strings.Index(imgFileName, ".")], "_", " ", -1)
			wordArr[i] = `<div class="thumb right"><a href="/img/`+ imgFileName +`">` + first + " " + second + " " + third + "</a><figcaption>"+figCaption+"</figcaption></div>"
		}

		// if we have an unordered list
		// * Item1
		// * Item2
		if reUnorderedList.MatchString(v) {
			wordArr[i] = reUnorderedList.ReplaceAllString(v, `<li>$1</li>`)
			if countUnOrdList == 0 {
				wordArr[i] = "<ul>" + wordArr[i]
			}
			match = true
			countUnOrdList++
		} else if match == true {
			wordArr[i-1] = wordArr[i-1] + "</ul>"
			match = false
			countUnOrdList = 0
		}

		// if we have an ordered list
		// # Item1
		// # Item2
		if reOrderedList.MatchString(v) {
			wordArr[i] = reOrderedList.ReplaceAllString(v, `<li>$1</li>`)
			if countOrdList == 0 {
				wordArr[i] = "<ol>" + wordArr[i]
			}
			matchOrd = true
			countOrdList++
		} else if matchOrd == true {
			wordArr[i-1] = wordArr[i-1] + "</ol>"
			matchOrd = false
			countOrdList = 0
		}

		// if we have a description list
		// ; Term : Definition1
		// : Definition2
		if reDescriptionList.MatchString(v) {
			var reSingleLine = regexp.MustCompile(`^;([^:]*):(.*)+`)
			if reSingleLine.MatchString(v) {
				wordArr[i] = reSingleLine.ReplaceAllString(v, `<dl><dt>$1</dt><dd>$2</dd></dl>`)
			} else {
				wordArr[i] = reDescriptionList.ReplaceAllString(v, `<dt>$1</dt>`)


				if countDList == 0 {
					wordArr[i] = "<dl>" + wordArr[i]
				}
				matchList = true
				countDList++
			}
		} else if matchList == true && reSubDescriptionList.MatchString(v) {
			wordArr[i] = reSubDescriptionList.ReplaceAllString(v, `<dd>$1</dd>`)
		} else if matchList == true {
			wordArr[i-1] = wordArr[i-1] + "</dl>"
			matchList = false
			countDList = 0
		}
	}

	// close list tags
	if(match == true) {
		wordArr[len(wordArr)-1] = wordArr[len(wordArr)-1] + "</ul>"
	}
	if(matchOrd == true) {
		wordArr[len(wordArr)-1] = wordArr[len(wordArr)-1] + "</ol>"
	}

	// bring split array back into a single string
	s = strings.Join(wordArr, "\n")

	// ---- to <hr>
	re = regexp.MustCompile(`----`)
	s = re.ReplaceAllString(s, `<hr>`)

	// ^\$ . $ to <blockquote> . </blockquote>
	re = regexp.MustCompile(`(?m)^\$ (.*)`)
	s = re.ReplaceAllString(s, `<blockquote>$1</blockquote>`)

	// *** . *** to <b><i> . </i></b>
	re = regexp.MustCompile(`\*\*\*(.*?)\*\*\*`)
	s = re.ReplaceAllString(s, `<b><i>$1</i></b>`)

	// ** . ** to <b> . </b>
	re = regexp.MustCompile(`\*\*(.*?)\*\*`)
	s = re.ReplaceAllString(s, `<b>$1</b>`)

	// * . * to <i> . </i>
	re = regexp.MustCompile(`\*(.*?)\*`)
	s = re.ReplaceAllString(s, `<i>$1</i>`)

	// youtbe url to <object></object>
	re = regexp.MustCompile(`https[:]\/\/www.youtube.com\/watch\?v=([a-zA-Z0-9_]*)`)
	s = re.ReplaceAllString(s, `<object style="width:100%;height:100%;width:420px;height:315px;float:none;clear:both;margin:2px auto;" data="http://www.youtube.com/embed/$1"></object>`)

	// replace email with mailto
	re = regexp.MustCompile(`\w+@\w+\.\w+(\.\w+)?`)
	s = re.ReplaceAllString(s, `<a href="mailto:$0">$0</a>`)

	// markdown force unwrap image (external)
	re = regexp.MustCompile(`!\[(.*?)\]\((.*?)\)`)
	s = re.ReplaceAllString(s, `<img src="/img/$2" alt="$1">`)

	// [[Internal URL|Caption]]
	re = regexp.MustCompile(`\[\[([^\]\[:]+)\|([^\]\[:]+)\]\]`)
	s = re.ReplaceAllString(s, `<a href="/entries/$1">$2</a>`)

	// regex string to catch ~most URLS (see https://mathiasbynens.be/demo/url-regex)
	urlReg := `(https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=;!]{2,256}\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_\+.~#?&//=;!]*))`

	re = regexp.MustCompile(`\[`+urlReg+`\s([^\]]+)\]`)
	s = re.ReplaceAllString(s, `<a class="external" href="$1">$4</a>`)

	re = regexp.MustCompile(`\[?`+urlReg+`\]?( |\n)`)
	s = re.ReplaceAllString(s, `<a class="external" href="$1">$1</a>$4`)

	re = regexp.MustCompile(`(?m)`+urlReg+`$`)
	s = re.ReplaceAllString(s, `<a class="external" href="$1">$1</a>`)

	re = regexp.MustCompile(`\[([a-z]+)\]\(([a-zA-Z0-9_\/:.-;!]+)\)`)
	s = re.ReplaceAllString(s, `<a class="external" href="$2">$1</a>`)

	re = regexp.MustCompile(`\[\[(.*?)\]\]`)
	s = re.ReplaceAllString(s, `<a href="/entries/$1">$1</a>`)

	//re = regexp.MustCompile(`([^"])(https?:\/\/(www\.)?[-a-zA-Z0-9@:%._\+~#=;!]{2,256}\.[a-z]{2,6}\b([-a-zA-Z0-9@:%_\+.~#?&//=;!]*))`)
	//s = re.ReplaceAllString(s, `$1<a class="external" href="$2">$2</a>`)

	// replace newlines with a newline+new paragraph tag
	s = strings.Replace(s, "\n", "\n<p>" , -1)

	// get rid of hanging paragraph tags
	re = regexp.MustCompile(`(?m)^<p>$`)
	s = re.ReplaceAllString(s, ``)

	// headers don't need to exist within a paragraph
	re = regexp.MustCompile(`<p>(<h[1-5].*)`)
	s = re.ReplaceAllString(s, `$1`)

	return s
}

func Header(s string) (string, [][]string) {
	// ='s to corresponding header tag
	var h5 = regexp.MustCompile(`=====(.*?)=====`)
	var h4 = regexp.MustCompile(`====(.*?)====`)
	var h3 = regexp.MustCompile(`===(.*?)===`)
	var h2 = regexp.MustCompile(`==(.*?)==`)

	// go ahead and replace those that are not part of the TOC
	s = h5.ReplaceAllString(s, `<h5 id="$1">$1</h5>`)
	s = h4.ReplaceAllString(s, `<h4 id="$1">$1</h4>`)

	// 2d array to hold the table of contents
	toc := [][]string{}
	// split string into array of words (for spaces)
	words := strings.Fields(s)
	counter := -1

	for i, word := range words {
		if len(word) > 4 {
			// if we have a header that contains a space...
			if word[0:2] == "==" && word[len(word)-2:len(word)] != "==" {
				// loop through the word list until we reach the end of the header
				for k:=i; word[len(word)-2:len(word)] != "=="; k++ {
					word += " " + words[k+1]
				}
			}
		}

		// if matched, add it to the table of contents
		if h3.MatchString(word) {
			if counter > -1 {
				toc[counter] = append(toc[counter], h3.FindStringSubmatch(word)[1])
			}
		} else if h2.MatchString(word) {
			toc = append(toc, []string{h2.FindStringSubmatch(word)[1]})
			counter++
		}
	}

	// finally, replace the remaining headers
	s = h3.ReplaceAllString(s, `<h3 id="$1">$1</h3>`)
	s = h2.ReplaceAllString(s, `<h2 id="$1">$1</h2>`)

	return s, toc
}

// replace emoticon markup with <img>
func Emoticons(s string) string {
	e_path := "<img src=/img/emoticons/"
	s = strings.Replace(s,":angry:",e_path + "angry.gif>",-1)
	//s = strings.Replace(s,">:(",e_path + "angry.gif>",-1)
	s = strings.Replace(s,":laugh:",e_path + "laugh.gif>",-1)
	s = strings.Replace(s,":DD",e_path + "laugh.gif>",-1)
	s = strings.Replace(s,":yell:",e_path + "yell.gif>",-1)
	//s = strings.Replace(s,">:O",e_path + "yell.gif>",-1)
	s = strings.Replace(s,":innocent:",e_path + "innocent.gif>",-1)
	s = strings.Replace(s,"O:)",e_path + "innocent.gif>",-1)
	s = strings.Replace(s,":satisfied:",e_path + "satisfied.gif>",-1)
	s = strings.Replace(s,"/:D",e_path + "satisfied.gif>",-1)
	s = strings.Replace(s,":)",e_path + "smile.gif>",-1)
	s = strings.Replace(s,":O",e_path + "shocked.gif>",-1)
	s = strings.Replace(s,":(",e_path + "sad.gif>",-1)
	s = strings.Replace(s,":D",e_path + "biggrin.gif>",-1)
	s = strings.Replace(s,":P",e_path + "tongue.gif>",-1)
	s = strings.Replace(s,";)",e_path + "wink.gif>",-1)
	s = strings.Replace(s,":blush:",e_path + "blush.gif>",-1)
	s = strings.Replace(s,":\\",e_path + "blush.gif>",-1)
	s = strings.Replace(s,":confused:",e_path + "confused.gif>",-1)
	s = strings.Replace(s,":S",e_path + "confused.gif>",-1)
	s = strings.Replace(s,":cool:",e_path + "cool.gif>",-1)
	s = strings.Replace(s,"B)",e_path + "cool.gif>",-1)
	s = strings.Replace(s,":crazy:",e_path + "crazy.gif>",-1)
	s = strings.Replace(s,":cry:",e_path + "cry.gif>",-1)
	s = strings.Replace(s,":~(",e_path + "cry.gif>",-1)
	s = strings.Replace(s,":doze",e_path + "doze.gif>",-1)
	s = strings.Replace(s,":?",e_path + "doze.gif>",-1)
	s = strings.Replace(s,":hehe:",e_path + "hehe.gif>",-1)
	s = strings.Replace(s,"XD",e_path + "hehe.gif>",-1)
	s = strings.Replace(s,":plain:",e_path + "plain.gif>",-1)
	s = strings.Replace(s,":|",e_path + "plain.gif>",-1)
	s = strings.Replace(s,":rolleyes:",e_path + "rolleyes.gif>",-1)
	s = strings.Replace(s,"9_9",e_path + "rolleyes.gif>",-1)
	s = strings.Replace(s,":dizzy:",e_path + "crazy.gif>",-1)
	s = strings.Replace(s,"o_O",e_path + "crazy.gif>",-1)
	s = strings.Replace(s,":money:",e_path + "money.gif>",-1)
	s = strings.Replace(s,":$",e_path + "money.gif>",-1)
	s = strings.Replace(s,":sealed:",e_path + "sealed.gif>",-1)
	s = strings.Replace(s,":X",e_path + "sealed.gif>",-1)
	s = strings.Replace(s,":eek:",e_path + "eek.gif>",-1)
	s = strings.Replace(s,"O_O",e_path + "eek.gif>",-1)
	s = strings.Replace(s,":kiss:",e_path + "kiss.gif>",-1)
	s = strings.Replace(s,":*",e_path + "kiss.gif>",-1)
	s = strings.Replace(s,"&lt;/p&gt;", "</p>", -1)
	s = strings.Replace(s,"&lt;p&gt;", "<p>", -1)

	return s
}
