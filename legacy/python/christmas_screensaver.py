"""Developer Christmas Tree — Tk desktop app and Windows screen saver."""
import ctypes
import json
import math
import os
import sys
import time
import tkinter as tk
from tkinter import ttk, colorchooser, messagebox
from pathlib import Path
from tree_engine import TREE, SNOW, rgb

APP = 'Developer Christmas Tree'
BG = '#071019'
PANEL = '#111d2b'
FG = '#e5edf7'
MUTED = '#9baec2'
ACCENT = '#56e1bd'
CONFIG = Path(os.getenv('LOCALAPPDATA') or Path.home() / '.config') / 'DeveloperChristmasTree' / 'settings.json'
DEFAULT = dict(theme='Rainbow RGB', speed=1.0, direction='Clockwise', fps=30, size=88,
               snow=True, ornaments=True, messages=True, custom='#5bd6ff', background=BG, light_speed=1.0)
THEMES = ('Rainbow RGB', 'Classic Christmas', 'Ice Blue', 'Cyberpunk', 'Gold & White', 'Matrix Green', 'Custom')
THEME_LABELS = ('무지개 RGB', '클래식 크리스마스', '아이스 블루', '사이버펑크', '골드 & 화이트', '매트릭스 그린', '사용자 지정')
DIRS = ('Clockwise', 'Counterclockwise', 'Static')
DIR_LABELS = ('시계 방향', '반시계 방향', '정지')

def valid_hex(s):
    return isinstance(s, str) and len(s)==7 and s[0]=='#' and all(c in '0123456789abcdefABCDEF' for c in s[1:])

def normalize(raw):
    result = DEFAULT.copy()
    if not isinstance(raw, dict): return result
    for k, options in [('theme', THEMES), ('direction', DIRS)]:
        if raw.get(k) in options: result[k] = raw[k]
    for k, lo, hi in [('speed',0,4), ('light_speed',0,4), ('fps',10,60), ('size',45,100)]:
        try:
            v=float(raw[k]); assert math.isfinite(v)
            result[k] = max(lo,min(hi,v))
            if k in ('fps','size'): result[k]=round(result[k])
        except (KeyError, TypeError, ValueError, AssertionError): pass
    for k in ('snow','ornaments','messages'):
        if isinstance(raw.get(k),bool): result[k]=raw[k]
    for k in ('custom','background'):
        if valid_hex(raw.get(k)): result[k]=raw[k]
    return result

def load_settings():
    try: return normalize(json.loads(CONFIG.read_text(encoding='utf-8')))
    except (OSError, ValueError): return DEFAULT.copy()

def save_settings(cfg):
    CONFIG.parent.mkdir(parents=True,exist_ok=True)
    temp=CONFIG.with_suffix('.tmp')
    temp.write_text(json.dumps(normalize(cfg),indent=2,ensure_ascii=False),encoding='utf-8')
    temp.replace(CONFIG)

def hx(c): return '#%02x%02x%02x'%c

def blend_hex(s, factor):
    return tuple(round(int(s[i:i+2],16)*factor) for i in (1,3,5))

def palette(p, shade, t, cfg):
    theme=cfg['theme']; u=p.hue; lt=t*cfg['light_speed']
    if p.kind=='trunk': return (153,91,52)
    if theme=='Custom':
        return blend_hex(cfg['custom'], min(1,.40+shade*.60) if p.kind=='leaf' else .80+.20*math.sin(lt*3+u*16)**2)
    if p.kind=='leaf':
        h,s={'Ice Blue':(.56,.74),'Cyberpunk':(.79,.79),'Gold & White':(.12,.52),'Matrix Green':(.32,.91)}.get(theme,(.36,.82))
        return rgb(h+.055*u,s,shade)
    if theme=='Rainbow RGB':
        if p.kind=='tinsel': return rgb(.13,.55,.98)
        return rgb(u*1.9-lt*.32,.76,1)
    if theme=='Classic Christmas': return rgb((.0,.12,.34)[int(u*100)%3],.82,1)
    if theme=='Ice Blue': return rgb(.53+.11*u,.30+.55*u,1)
    if theme=='Cyberpunk': return rgb(.49 if int(u*9)%2 else .86,.85,1)
    if theme=='Gold & White': return rgb(.12,.12+.70*u,1)
    return rgb(.30+.06*u,.85,1)

def render_cells(cols, rows, t, angle, cfg):
    cells={}
    def put(x,y,ch,col,d=1e6):
        if 0<=x<cols and 0<=y<rows and ((x,y) not in cells or d>=cells[(x,y)][2]):
            cells[(x,y)]=(ch,col,d)
    if cols<18 or rows<12: return cells
    def label(y, text, color):
        for i,ch in enumerate(text):
            if ch!=' ': put((cols-len(text))//2+i,y,ch,color)
    scale=min((rows-7)/3.12,(cols-4)/4.65)*cfg['size']/100
    cx=(cols-1)/2; cy=(rows-1)/2+.2
    ca,sa=math.cos(angle),math.sin(angle)
    ct,st=math.cos(.12),math.sin(.12)
    def project(x,y,z):
        depth=z*ct+y*st; v=y*ct-z*st; persp=5/(5-depth)
        return round(cx+x*scale*1.86*persp),round(cy-v*scale*persp),depth
    if cfg['snow']:
        for i,(sx,sy,vel) in enumerate(SNOW[:min(len(SNOW),cols*rows//20)]):
            x=int(((sx+.009*math.sin(t+i))%1)*cols)
            y=int(((sy+t*vel/5)%1)*rows)
            put(x,y,'.' if i%4 else '*',(100,139,174),-100)
    for p in TREE:
        if p.kind=='ornament' and not cfg['ornaments']: continue
        x,z=p.x*ca-p.z*sa,p.x*sa+p.z*ca
        px,py,d=project(x,p.y,z)
        # Skip hidden points before calculating colors; foreground is selected by depth.
        if not (0<=px<cols and 0<=py<rows) or ((px,py) in cells and cells[(px,py)][2]>d): continue
        shade=.34+.48*(z/max(.035,math.hypot(x,z))+1)/2+.14*p.hue
        ch=p.char
        if p.kind=='light': ch='O' if math.sin(t*cfg['light_speed']*6+p.hue*40)>.45 else 'o'
        put(px,py,ch,palette(p,min(1,shade),t,cfg),d)
    sx,sy,_=project(0,1.14,0)
    star=rgb(.13,.47,.87+.13*math.sin(t*3)**2)
    for dx,dy,ch in ((0,-2,'|'),(-1,-1,'\\'),(0,-1,'*'),(1,-1,'/'),(-2,0,'-'),(-1,0,'-'),(0,0,'@'),(1,0,'-'),(2,0,'-'),(-1,1,'/'),(1,1,'\\')):
        put(sx+dx,sy+dy,ch,star)
    if cfg['messages']:
        title='M E R R Y   C H R I S T M A S' if cols>=32 else 'MERRY CHRISTMAS'
        for i,ch in enumerate(title):
            if ch!=' ': put((cols-len(title))//2+i,1,ch,rgb(i/20-t*cfg['light_speed']*.13,.55,1) if cfg['theme']=='Rainbow RGB' else (226,235,248))
        label(rows-3,'<<< HAPPY CODING >>>',(255,217,131))
        msg=('while True: spread_joy()', 'git commit -m "Christmas"', 'print("Hello, Christmas!")', '404: bugs not found')
        label(rows-2,msg[int(t/4)%len(msg)],(130,217,245))
    return cells

class Animation:
    def __init__(self,canvas,get_cfg):
        self.canvas=canvas; self.get_cfg=get_cfg; self.items={}; self.old={}; self.layout=None
        self.start=self.last=time.monotonic(); self.angle=0; self.job=None; self.running=True; self.frames=0
        self.tick()
    def tick(self):
        if not self.running: return
        now=time.monotonic(); dt=min(.2,now-self.last); self.last=now; cfg=self.get_cfg()
        sign={'Clockwise':1,'Counterclockwise':-1,'Static':0}[cfg['direction']]
        self.angle=(self.angle+dt*.85*cfg['speed']*sign)%(2*math.pi)
        w,h=self.canvas.winfo_width(),self.canvas.winfo_height()
        if w>20 and h>20:
            # Pixel font sizing keeps ASCII grid proportional at different DPIs.
            fs=max(5, min(26,int(min(h/46,w/68))))
            cw=fs*.62; rh=fs*1.16
            cols=max(1,min(160,int(w/cw))); rows=max(1,min(65,int(h/rh)))
            layout=(w,h,fs,cols,rows,cfg['background'])
            if layout!=self.layout:
                self.canvas.delete('all'); self.items.clear(); self.old.clear(); self.layout=layout
                self.canvas.configure(bg=cfg['background'])
            cells=render_cells(cols,rows,now-self.start,self.angle,cfg)
            for key in self.old.keys()-cells.keys(): self.canvas.itemconfigure(self.items[key],state='hidden')
            for key,(ch,color,_) in cells.items():
                value=(ch,hx(color)); old=self.old.get(key)
                if key not in self.items:
                    x,y=key
                    self.items[key]=self.canvas.create_text(w/2+(x-(cols-1)/2)*cw,h/2+(y-(rows-1)/2)*rh,text=ch,fill=value[1],font=('Consolas',-fs,'bold'))
                elif old!=value:
                    self.canvas.itemconfigure(self.items[key],text=ch,fill=value[1],state='normal')
            self.old={k:(v[0],hx(v[1])) for k,v in cells.items()}; self.frames+=1
        elapsed=time.monotonic()-now
        self.job=self.canvas.after(max(8,round(1000/cfg['fps']-elapsed*1000)),self.tick)
    def stop(self):
        self.running=False
        if self.job is not None:
            try: self.canvas.after_cancel(self.job)
            except tk.TclError: pass


def make_canvas(root): return tk.Canvas(root,highlightthickness=0,bg=BG)

def full_screen(root,get_cfg,return_to_settings=False):
    win=tk.Toplevel(root) if return_to_settings else root
    win.configure(bg=BG,cursor='none'); win.attributes('-fullscreen',True); win.attributes('-topmost',True)
    canvas=make_canvas(win); canvas.pack(fill='both',expand=True)
    anim=Animation(canvas,get_cfg); launched=time.monotonic(); origin=[None]; closed=[False]
    def close(event=None):
        if closed[0]: return
        closed[0]=True; anim.stop(); win.destroy()
    def motion(e):
        if time.monotonic()-launched<.75: return
        p=(e.x_root,e.y_root)
        if origin[0] is None: origin[0]=p
        elif math.hypot(p[0]-origin[0][0],p[1]-origin[0][1])>=12: close()
    win.bind('<KeyPress>',close); win.bind('<ButtonPress>',close); win.bind('<Motion>',motion)
    win.protocol('WM_DELETE_WINDOW',close); win.focus_force()
    return anim


def settings_ui(root):
    root.title(APP+' | 설정'); root.geometry('1050x740'); root.minsize(790,650); root.configure(bg=BG)
    cfg=load_settings(); live=cfg.copy()
    style=ttk.Style(root); style.theme_use('clam')
    style.configure('.',background=PANEL,foreground=FG,font=('Malgun Gothic',10))
    style.configure('TButton',background='#25384a',foreground=FG,padding=8,borderwidth=0)
    style.map('TButton',background=[('active','#37536a')]); style.configure('Accent.TButton',background='#147d68',foreground='white')
    style.map('Accent.TButton',background=[('active','#1b9b80')])
    style.configure('TCheckbutton',background=PANEL,foreground=FG); style.map('TCheckbutton',background=[('active',PANEL)])
    style.configure('TCombobox',fieldbackground='#223447',background='#223447',foreground=FG,arrowcolor=FG)
    style.map('TCombobox',fieldbackground=[('readonly','#223447')],foreground=[('readonly',FG)])
    style.configure('Horizontal.TScale',background=PANEL,troughcolor='#273b4c')
    header=tk.Frame(root,bg=BG); header.pack(fill='x',padx=24,pady=(18,13))
    tk.Label(header,text='DEVELOPER / CHRISTMAS TREE',font=('Consolas',22,'bold'),fg=ACCENT,bg=BG).pack(anchor='w')
    tk.Label(header,text='3D ASCII SCREENSAVER   /   LIVE CONFIGURATION',font=('Consolas',11),fg=MUTED,bg=BG).pack(anchor='w')
    body=tk.Frame(root,bg=BG); body.pack(fill='both',expand=True,padx=24,pady=(0,12)); body.grid_columnconfigure(0,weight=1); body.grid_rowconfigure(0,weight=1)
    preview=tk.Frame(body,bg=BG,highlightbackground='#294052',highlightthickness=1); preview.grid(row=0,column=0,sticky='nsew',padx=(0,14))
    tk.Label(preview,text='LIVE PREVIEW  •  실시간 미리보기',fg=ACCENT,bg=BG,font=('Malgun Gothic',10,'bold'),anchor='w').pack(fill='x',padx=12,pady=8)
    canvas=make_canvas(preview); canvas.pack(fill='both',expand=True,padx=2,pady=2)
    panel=tk.Frame(body,bg=PANEL,width=310); panel.grid(row=0,column=1,sticky='nsew'); panel.grid_propagate(False); panel.pack_propagate(False)
    inner=tk.Frame(panel,bg=PANEL); inner.pack(fill='both',expand=True,padx=16,pady=12)
    vars={}
    def heading(text): tk.Label(inner,text=text,bg=PANEL,fg=ACCENT,font=('Malgun Gothic',11,'bold')).pack(anchor='w',pady=(4,6))
    def select(k,label,options,labels):
        tk.Label(inner,text=label,bg=PANEL,fg=FG).pack(anchor='w')
        var=tk.StringVar(value=labels[options.index(live[k])]); vars[k]=(var,options,labels)
        box=ttk.Combobox(inner,textvariable=var,values=labels,state='readonly'); box.pack(fill='x',pady=(2,8))
        var.trace_add('write',lambda *_: live.update({k:options[labels.index(var.get())]}) if var.get() in labels else None)
    def slider(k,label,lo,hi,integer=False):
        row=tk.Frame(inner,bg=PANEL); row.pack(fill='x')
        tk.Label(row,text=label,bg=PANEL,fg=FG).pack(side='left')
        val=tk.StringVar(); tk.Label(row,textvariable=val,bg=PANEL,fg=ACCENT,font=('Consolas',10,'bold')).pack(side='right')
        var=tk.DoubleVar(value=live[k]); vars[k]=(var,None,None)
        def changed(*_):
            n=round(var.get()) if integer else round(var.get(),1); live[k]=n
            val.set(str(n) if integer else f'{n:.1f}x')
        var.trace_add('write',changed); changed()
        ttk.Scale(inner,from_=lo,to=hi,variable=var).pack(fill='x',pady=(0,5))
    def check(k,label):
        var=tk.BooleanVar(value=live[k]); vars[k]=(var,None,None)
        var.trace_add('write',lambda *_:live.update({k:var.get()})); ttk.Checkbutton(inner,text=label,variable=var).pack(anchor='w')
    def pick(k):
        result=colorchooser.askcolor(color=live[k],parent=root)[1]
        if result:
            live[k]=result; swatches[k].configure(bg=result)
            if k=='custom': vars['theme'][0].set(THEME_LABELS[-1])
    heading('01  /  컬러 및 조명'); select('theme','색상 테마',THEMES,THEME_LABELS)
    swatches={}; colors=tk.Frame(inner,bg=PANEL); colors.pack(fill='x',pady=(0,5))
    for k,label in [('custom','직접 색상'),('background','배경 색상')]:
        cell=tk.Frame(colors,bg=PANEL); cell.pack(side='left',expand=True,fill='x')
        swatches[k]=tk.Button(cell,bg=live[k],activebackground=live[k],width=2,relief='flat',command=lambda k=k:pick(k)); swatches[k].pack(side='left')
        tk.Label(cell,text=label,bg=PANEL,fg=MUTED,font=('Malgun Gothic',9)).pack(side='left',padx=4)
    slider('light_speed','조명 변화 속도',0,4)
    heading('02  /  모션'); select('direction','회전 방향',DIRS,DIR_LABELS); slider('speed','회전 속도',0,4); slider('size','트리 크기 (%)',45,100,True); slider('fps','목표 FPS',10,60,True)
    heading('03  /  효과'); check('snow','눈 내리는 효과'); check('ornaments','개발자 코드 장식'); check('messages','크리스마스 메시지')
    anim=Animation(canvas,lambda:live)
    footer=tk.Frame(root,bg=BG); footer.pack(fill='x',padx=24,pady=(0,18))
    status=tk.StringVar(value='설정 변경 사항은 즉시 미리보기에 적용됩니다.'); tk.Label(footer,textvariable=status,bg=BG,fg=MUTED,font=('Malgun Gothic',9)).pack(side='left')
    def save():
        try: save_settings(live); status.set('설정을 저장했습니다.')
        except OSError as e: messagebox.showerror(APP,str(e),parent=root)
    def reset():
        live.update(DEFAULT)
        for k,(var,opts,labels) in vars.items(): var.set(labels[opts.index(DEFAULT[k])] if opts else DEFAULT[k])
        for k,b in swatches.items(): b.configure(bg=live[k])
        status.set('기본값으로 복원했습니다. 저장 버튼을 눌러 적용하세요.')
    def show_full():
        anim.stop()
        fs=full_screen(root,lambda:live,True)
        fs.canvas.winfo_toplevel().bind('<Destroy>',lambda e:resume(e,fs),add='+')
    def resume(e,fs):
        # The canvas is destroyed before its top-level; restart only once.
        if e.widget is fs.canvas and not anim.running:
            anim.running=True; anim.last=time.monotonic(); anim.tick()
    ttk.Button(footer,text='설정 저장',style='Accent.TButton',command=save).pack(side='right',padx=(6,0))
    ttk.Button(footer,text='전체 화면',command=show_full).pack(side='right',padx=6)
    ttk.Button(footer,text='기본값',command=reset).pack(side='right',padx=6)
    def close(): anim.stop(); root.destroy()
    root.protocol('WM_DELETE_WINDOW',close)
    return anim,live


def embed_preview(root,hwnd):
    if os.name!='nt' or not hwnd: root.destroy(); return
    from ctypes import wintypes as wt
    u=ctypes.WinDLL('user32',use_last_error=True)
    for name, args, ret in [('IsWindow',[wt.HWND],wt.BOOL),('GetParent',[wt.HWND],wt.HWND),('SetParent',[wt.HWND,wt.HWND],wt.HWND),('GetWindowLongW',[wt.HWND,ctypes.c_int],ctypes.c_long),('SetWindowLongW',[wt.HWND,ctypes.c_int,ctypes.c_long],ctypes.c_long),('GetClientRect',[wt.HWND,ctypes.POINTER(wt.RECT)],wt.BOOL),('SetWindowPos',[wt.HWND,wt.HWND,ctypes.c_int,ctypes.c_int,ctypes.c_int,ctypes.c_int,wt.UINT],wt.BOOL)]:
        fn=getattr(u,name); fn.argtypes=args; fn.restype=ret
    if not u.IsWindow(hwnd): root.destroy(); return
    root.overrideredirect(True); root.geometry('1x1+0+0'); root.update_idletasks()
    child=root.winfo_id(); wrapper=u.GetParent(child) or child
    style=u.GetWindowLongW(wrapper,-16); u.SetWindowLongW(wrapper,-16,(style & ~0x80000000)|0x40000000)
    ctypes.set_last_error(0); prev=u.SetParent(wrapper,hwnd)
    if not prev and ctypes.get_last_error(): root.destroy(); return
    canvas=make_canvas(root); canvas.pack(fill='both',expand=True); cfg=load_settings(); anim=Animation(canvas,lambda:cfg)
    def resize():
        if not u.IsWindow(hwnd): anim.stop(); root.destroy(); return
        rect=wt.RECT()
        if u.GetClientRect(hwnd,ctypes.byref(rect)):
            u.SetWindowPos(wrapper,None,0,0,max(1,rect.right),max(1,rect.bottom),0x14)
        root.after(300,resize)
    resize()


def parse_mode(args, executable):
    if not args: return ('s' if executable.lower().endswith('.scr') else 'c',None)
    a=args[0].lower(); a={'--fullscreen':'/s','--settings':'/c'}.get(a,a)
    switch,_,suffix=a.partition(':'); switch=switch.lstrip('/-')
    if switch=='p':
        try: return 'p',int(suffix or args[1],0)
        except (ValueError,IndexError): return 'p',None
    return ('s' if switch=='s' else 'c',None)

def main():
    mode,handle=parse_mode(sys.argv[1:],sys.argv[0]); root=tk.Tk()
    if mode=='s':
        cfg=load_settings(); full_screen(root,lambda:cfg)
    elif mode=='p': embed_preview(root,handle)
    else: settings_ui(root)
    try: root.mainloop()
    except KeyboardInterrupt: pass

if __name__=='__main__': main()
