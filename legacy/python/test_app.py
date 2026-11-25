import tempfile, time, unittest
from pathlib import Path
import christmas_screensaver as app

class Tests(unittest.TestCase):
    def test_settings(self):
        self.assertEqual(app.normalize({'fps': 999, 'speed': -1,'theme':'bad','custom':'bad'})['fps'],60)
        self.assertEqual(app.normalize({'speed':float('nan')})['speed'],1)
        old=app.CONFIG
        try:
            with tempfile.TemporaryDirectory() as d:
                app.CONFIG=Path(d)/'nested'/'settings.json'
                c=app.DEFAULT | {'direction':'Counterclockwise','theme':'Ice Blue','speed':2.5}
                app.save_settings(c); self.assertEqual(app.load_settings(),c)
                app.CONFIG.write_text('{bad'); self.assertEqual(app.load_settings(),app.DEFAULT)
        finally: app.CONFIG=old
    def test_modes(self):
        for args,exe,out in [([], 'a.exe',('c',None)),([], 'a.scr',('s',None)),(['/s'],'a.scr',('s',None)),(['/C:12'],'a.scr',('c',None)),(['/p','123'],'a.scr',('p',123)),(['/p:123'],'a.scr',('p',123)),(['/p'],'a.scr',('p',None))]:
            self.assertEqual(app.parse_mode(args,exe),out)
    def test_render(self):
        for theme in app.THEMES:
            cfg=app.DEFAULT|{'theme':theme}
            a=app.render_cells(100,48,1,0,cfg); b=app.render_cells(100,48,1,1,cfg)
            self.assertNotEqual(a,b); self.assertGreater(len(a),100)
            self.assertTrue(all(0<=x<100 and 0<=y<48 and all(0<=i<=255 for i in v[1]) for (x,y),v in a.items()))
        self.assertEqual(app.render_cells(5,5,0,0,app.DEFAULT),{})
    def test_ui(self):
        root=app.tk.Tk(); anim,live=app.settings_ui(root)
        def pump(ms):
            root.after(ms, root.quit); root.mainloop()
        pump(600)
        self.assertGreater(anim.frames,1)
        def walk(w):
            for child in w.winfo_children():
                yield child
                yield from walk(child)
        buttons={w.cget('text'):w for w in walk(root) if isinstance(w,app.ttk.Button)}
        self.assertIn('전체 화면',buttons)
        live['speed']=3; buttons['기본값'].invoke(); self.assertEqual(live,app.DEFAULT)
        buttons['전체 화면'].invoke(); pump(200); self.assertFalse(anim.running)
        win=next(w for w in root.winfo_children() if isinstance(w,app.tk.Toplevel))
        win.event_generate('<KeyPress-Escape>'); pump(100)
        if win.winfo_exists(): win.destroy(); pump(100)
        self.assertTrue(anim.running)
        anim.stop(); root.destroy()

if __name__=='__main__': unittest.main(verbosity=2)
