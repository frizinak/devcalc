# DevCalc

Downloads and caches data from https://www.digitaltruth.com/devchart.php.

Allows you to quickly lookup/calculate developing times/notes/volumes for a
given developer and film stock combination.

Also supports mixing chemicals by weight if you specify the density of a given
chemical.

## Usage

### List all developers

`devcalc mdc list developers`

### List all film stocks

`devcalc mdc list stocks`

### Get developing info for rodinal and kentmere film stocks

`devcalc mdc get rodinal 'kent*'`

```
    50) kentmere100 1+50 20.0C [135: 9m] [120: 9m]
   100) kentmere100 1+25 20.0C [135: 9m] [120: 9m]
   100) kentmere100 1+50 20.0C [135: 15m] [120: 15m]
   200) kentmere100 1+50 20.0C [135: 20m] [120: 20m]
        Agitation: continuous first 30 secs, then 1-2 inversions per min.
   800) kentmere100 1+100 20.0C [135: 2h] [120: 2h]
        Stand development: agitate for first minute only, then allow to stand undisturbed
    50) kentmere400 1+50 20.0C [135: 10m30s] [120: 10m30s]
   100) kentmere400 1+50 20.0C [135: 13m] [120: 13m]
   200) kentmere400 1+50 20.0C [135: 15m] [120: 15m]
   400) kentmere400 1+25 20.0C [135: 7m30s] [120: 7m30s]
   400) kentmere400 1+50 20.0C [135: 17m30s] [120: 17m30s]
   800) kentmere400 1+25 20.0C [135: 9m] [120: 9m]
   800) kentmere400 1+50 20.0C [135: 19m] [120: 19m]
  1600) kentmere400 1+100 20.0C [135: 2h] [120: 2h]
        Stand development: agitate for first minute only, then allow to stand undisturbed
  1600) kentmere400 1+25 20.0C [135: 25m] [120: 25m]
```

### Calculate how much rodinal I need for a 500ml tank stand dev

`devcalc calc rodinal 1+100 500`

`4.95ml + 495ml = 500.00ml`

### Alias adox.adonal to rodinal and store its density

(I weighed 280g of 200ml of my batch of Adox Adonal)

`devcalc alias adox.adonal rodinal 280/200`

### Calculate how much Adonal I need to add to my Paterson tank for a roll of 135 in 1+25

`devcalc calc adox.adonal 1+25 280`

`10.77ml (15.08g) + 269ml = 280.00ml (284.31g)`

### Get everything I need to develop my roll of kentmere400 in Adonal at 1+25

`devcalc calc adox.adonal 1+25 280 kentmere400`

```
10.77ml (15.08g) + 269ml = 280.00ml (284.31g)
   100) kentmere100 1+25 20.0C [135: 9m] [120: 9m]
   400) kentmere400 1+25 20.0C [135: 7m30s] [120: 7m30s]
   800) kentmere400 1+25 20.0C [135: 9m] [120: 9m]
  1600) kentmere400 1+25 20.0C [135: 25m] [120: 25m]
```
